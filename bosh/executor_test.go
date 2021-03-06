package bosh_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-bootloader/bosh"
	"github.com/cloudfoundry/bosh-bootloader/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executor", func() {
	Describe("JumpboxCreateEnvArgs", func() {
		var (
			cmd *fakes.BOSHCommand

			stateDir              string
			relativeDeploymentDir string
			relativeVarsDir       string

			executor         bosh.Executor
			interpolateInput bosh.InterpolateInput
		)

		BeforeEach(func() {
			cmd = &fakes.BOSHCommand{}
			cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
				stdout.Write([]byte("some-manifest"))
				return nil
			}
			cmd.GetBOSHPathCall.Returns.Path = "bosh-path"

			var err error
			stateDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			deploymentDir := filepath.Join(stateDir, "deployment")
			err = os.Mkdir(deploymentDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			varsDir := filepath.Join(stateDir, "vars")
			err = os.Mkdir(varsDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			relativeDeploymentDir = "${BBL_STATE_DIR}/deployment"
			relativeVarsDir = "${BBL_STATE_DIR}/vars"

			interpolateInput = bosh.InterpolateInput{
				IAAS:          "aws",
				DeploymentDir: deploymentDir,
				VarsDir:       varsDir,
				StateDir:      stateDir,
				BOSHState: map[string]interface{}{
					"key": "value",
				},
				Variables: "key: value",
				OpsFile:   "some-ops-file",
			}

			executor = bosh.NewExecutor(cmd, ioutil.ReadFile, json.Unmarshal, json.Marshal, ioutil.WriteFile)
		})

		It("generates create-env args for jumpbox", func() {
			interpolateInput.DeploymentVars = "internal_cidr: 10.0.0.0/24"
			interpolateInput.OpsFile = ""

			err := executor.JumpboxCreateEnvArgs(interpolateInput)
			Expect(err).NotTo(HaveOccurred())

			expectedArgs := []string{
				fmt.Sprintf("%s/jumpbox.yml", relativeDeploymentDir),
				"--state", fmt.Sprintf("%s/jumpbox-state.json", relativeVarsDir),
				"--vars-store", fmt.Sprintf("%s/jumpbox-variables.yml", relativeVarsDir),
				"--vars-file", fmt.Sprintf("%s/jumpbox-deployment-vars.yml", relativeVarsDir),
				"-o", fmt.Sprintf("%s/cpi.yml", relativeDeploymentDir),
			}

			By("writing the create-env args to a shell script", func() {
				expectedScript := formatScript("create-env", stateDir, expectedArgs)
				shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/create-jumpbox.sh", stateDir))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(shellScript)).To(Equal(expectedScript))
			})

			By("writing the delete-env args to a shell script", func() {
				expectedScript := formatScript("delete-env", stateDir, expectedArgs)
				shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/delete-jumpbox.sh", stateDir))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(shellScript)).To(Equal(expectedScript))
			})
		})

		Context("when a create-env script already exists", func() {
			var (
				createEnvPath     string
				createEnvContents string
			)
			BeforeEach(func() {
				createEnvPath = filepath.Join(stateDir, "create-jumpbox.sh")
				createEnvContents = "#!/bin/bash\necho 'I already exist'\n"

				ioutil.WriteFile(createEnvPath, []byte(createEnvContents), os.ModePerm)
			})

			It("does not override the existing script", func() {
				err := executor.JumpboxCreateEnvArgs(interpolateInput)
				Expect(err).NotTo(HaveOccurred())

				shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/create-jumpbox.sh", stateDir))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(shellScript)).To(Equal(createEnvContents))
			})
		})

		Context("when a delete-env script already exists", func() {
			var (
				deleteEnvPath     string
				deleteEnvContents string
			)
			BeforeEach(func() {
				deleteEnvPath = filepath.Join(stateDir, "delete-jumpbox.sh")
				deleteEnvContents = "#!/bin/bash\necho 'I already exist'\n"

				ioutil.WriteFile(deleteEnvPath, []byte(deleteEnvContents), os.ModePerm)
			})

			It("does not override the existing script", func() {
				err := executor.JumpboxCreateEnvArgs(interpolateInput)
				Expect(err).NotTo(HaveOccurred())

				shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/delete-jumpbox.sh", stateDir))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(shellScript)).To(Equal(deleteEnvContents))
			})
		})
	})

	Describe("DirectorCreateEnvArgs", func() {
		var (
			cmd *fakes.BOSHCommand

			stateDir              string
			relativeDeploymentDir string
			relativeVarsDir       string

			executor         bosh.Executor
			interpolateInput bosh.InterpolateInput
		)

		BeforeEach(func() {
			cmd = &fakes.BOSHCommand{}
			cmd.GetBOSHPathCall.Returns.Path = "bosh-path"

			var err error
			stateDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			deploymentDir := filepath.Join(stateDir, "deployment")
			err = os.Mkdir(deploymentDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			varsDir := filepath.Join(stateDir, "vars")
			err = os.Mkdir(varsDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			relativeDeploymentDir = "${BBL_STATE_DIR}/deployment"
			relativeVarsDir = "${BBL_STATE_DIR}/vars"

			interpolateInput = bosh.InterpolateInput{
				DeploymentDir:  deploymentDir,
				StateDir:       stateDir,
				VarsDir:        varsDir,
				DeploymentVars: "internal_cidr: 10.0.0.0/24",
				BOSHState: map[string]interface{}{
					"key": "value",
				},
				Variables: "key: value",
				OpsFile:   "some-ops-file",
			}

			executor = bosh.NewExecutor(cmd, ioutil.ReadFile, json.Unmarshal, json.Marshal, ioutil.WriteFile)
		})

		Context("azure", func() {
			var azureInterpolateInput bosh.InterpolateInput

			BeforeEach(func() {
				azureInterpolateInput = interpolateInput
				azureInterpolateInput.IAAS = "azure"
			})

			It("generates a bosh manifest", func() {
				cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
					stdout.Write([]byte("some-manifest"))
					return nil
				}

				err := executor.DirectorCreateEnvArgs(azureInterpolateInput)
				Expect(err).NotTo(HaveOccurred())
				Expect(cmd.RunCallCount()).To(Equal(0))

				expectedArgs := []string{
					fmt.Sprintf("%s/bosh.yml", relativeDeploymentDir),
					"--state", fmt.Sprintf("%s/bosh-state.json", relativeVarsDir),
					"--vars-store", fmt.Sprintf("%s/director-variables.yml", relativeVarsDir),
					"--vars-file", fmt.Sprintf("%s/director-deployment-vars.yml", relativeVarsDir),
					"-o", fmt.Sprintf("%s/cpi.yml", relativeDeploymentDir),
					"-o", fmt.Sprintf("%s/jumpbox-user.yml", relativeDeploymentDir),
					"-o", fmt.Sprintf("%s/uaa.yml", relativeDeploymentDir),
					"-o", fmt.Sprintf("%s/credhub.yml", relativeDeploymentDir),
					"-o", fmt.Sprintf("%s/user-ops-file.yml", relativeVarsDir),
				}

				By("writing the create-env args to a shell script", func() {
					expectedScript := formatScript("create-env", stateDir, expectedArgs)
					shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/create-director.sh", stateDir))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(shellScript)).To(Equal(expectedScript))
				})

				By("writing the delete-env args to a shell script", func() {
					expectedScript := formatScript("delete-env", stateDir, expectedArgs)
					shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/delete-director.sh", stateDir))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(shellScript)).To(Equal(expectedScript))
				})
			})
		})

		Context("gcp", func() {
			var gcpInterpolateInput bosh.InterpolateInput

			BeforeEach(func() {
				gcpInterpolateInput = interpolateInput
				gcpInterpolateInput.IAAS = "gcp"

				cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
					stdout.Write([]byte("some-manifest"))
					return nil
				}
			})

			Context("when a create-env script already exists", func() {
				var (
					createEnvPath     string
					createEnvContents string
				)
				BeforeEach(func() {
					createEnvPath = filepath.Join(stateDir, "create-director.sh")
					createEnvContents = "#!/bin/bash\necho 'I already exist'\n"

					ioutil.WriteFile(createEnvPath, []byte(createEnvContents), os.ModePerm)
				})

				It("does not override the existing script", func() {
					err := executor.DirectorCreateEnvArgs(gcpInterpolateInput)
					Expect(err).NotTo(HaveOccurred())

					shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/create-director.sh", stateDir))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(shellScript)).To(Equal(createEnvContents))
				})
			})

			Context("when a delete-env script already exists", func() {
				var (
					deleteEnvPath     string
					deleteEnvContents string
				)
				BeforeEach(func() {
					deleteEnvPath = filepath.Join(stateDir, "delete-director.sh")
					deleteEnvContents = "#!/bin/bash\necho 'I already exist'\n"

					ioutil.WriteFile(deleteEnvPath, []byte(deleteEnvContents), os.ModePerm)
				})

				It("does not override the existing script", func() {
					err := executor.DirectorCreateEnvArgs(gcpInterpolateInput)
					Expect(err).NotTo(HaveOccurred())

					shellScript, err := ioutil.ReadFile(fmt.Sprintf("%s/delete-director.sh", stateDir))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(shellScript)).To(Equal(deleteEnvContents))
				})
			})
		})
	})

	Describe("CreateEnv", func() {
		var (
			cmd      *fakes.BOSHCommand
			executor bosh.Executor

			createEnvPath string
			varsDir       string
			stateDir      string

			createEnvInput bosh.CreateEnvInput
		)

		BeforeEach(func() {
			var err error

			cmd = &fakes.BOSHCommand{}
			varsDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			stateDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			executor = bosh.NewExecutor(cmd, ioutil.ReadFile, json.Unmarshal, json.Marshal, ioutil.WriteFile)

			createEnvInput = bosh.CreateEnvInput{
				Deployment: "some-deployment",
				StateDir:   stateDir,
				VarsDir:    varsDir,
			}

			createEnvPath = filepath.Join(stateDir, "create-some-deployment.sh")
			createEnvContents := fmt.Sprintf("#!/bin/bash\necho 'some-vars-store-contents' > %s/some-deployment-variables.yml\n", varsDir)

			ioutil.WriteFile(createEnvPath, []byte(createEnvContents), os.ModePerm)
		})

		AfterEach(func() {
			os.Remove(filepath.Join(varsDir, "some-deployment-variables.yml"))
			os.Remove(filepath.Join(stateDir, "create-some-deployment.sh"))
			os.Unsetenv("BBL_STATE_DIR")
		})

		It("runs the create-env script and returns the resulting vars-store contents", func() {
			vars, err := executor.CreateEnv(createEnvInput)
			Expect(err).NotTo(HaveOccurred())

			Expect(cmd.RunCallCount()).To(Equal(0))
			Expect(vars).To(ContainSubstring("some-vars-store-contents"))

			By("setting BBL_STATE_DIR environment variable", func() {
				bblStateDirEnv := os.Getenv("BBL_STATE_DIR")
				Expect(bblStateDirEnv).To(Equal(stateDir))
			})
		})

		Context("when the create-env script returns an error", func() {
			BeforeEach(func() {
				createEnvContents := "#!/bin/bash\nexit 1\n"
				ioutil.WriteFile(createEnvPath, []byte(createEnvContents), os.ModePerm)
			})

			It("returns an error", func() {
				vars, err := executor.CreateEnv(createEnvInput)
				Expect(err).To(MatchError("Run bosh create-env: exit status 1"))
				Expect(vars).To(Equal(""))
			})
		})
	})

	Describe("DeleteEnv", func() {
		var (
			cmd      *fakes.BOSHCommand
			executor bosh.Executor

			deleteEnvPath string
			varsDir       string
			stateDir      string

			deleteEnvInput bosh.DeleteEnvInput
		)

		BeforeEach(func() {
			var err error
			cmd = &fakes.BOSHCommand{}
			varsDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			stateDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			executor = bosh.NewExecutor(cmd, ioutil.ReadFile, json.Unmarshal, json.Marshal, ioutil.WriteFile)

			deleteEnvInput = bosh.DeleteEnvInput{
				Deployment: "some-deployment",
				VarsDir:    varsDir,
				StateDir:   stateDir,
			}

			deleteEnvPath = filepath.Join(stateDir, "delete-some-deployment.sh")
			deleteEnvContents := "#!/bin/bash\necho delete-env > /dev/null\n"

			ioutil.WriteFile(deleteEnvPath, []byte(deleteEnvContents), os.ModePerm)
		})

		AfterEach(func() {
			os.Unsetenv("BBL_STATE_DIR")
			os.Remove(filepath.Join(stateDir, "delete-some-deployment.sh"))
		})

		It("deletes a bosh environment with the delete-env script", func() {
			err := executor.DeleteEnv(deleteEnvInput)
			Expect(err).NotTo(HaveOccurred())

			Expect(cmd.RunCallCount()).To(Equal(0))

			By("setting BBL_STATE_DIR environment variable", func() {
				bblStateDirEnv := os.Getenv("BBL_STATE_DIR")
				Expect(bblStateDirEnv).To(Equal(stateDir))
			})
		})

		Context("when the create-env script returns an error", func() {
			BeforeEach(func() {
				deleteEnvContents := "#!/bin/bash\nexit 1\n"
				ioutil.WriteFile(deleteEnvPath, []byte(deleteEnvContents), os.ModePerm)
			})

			It("returns an error", func() {
				err := executor.DeleteEnv(deleteEnvInput)
				Expect(err).To(MatchError("Run bosh delete-env: exit status 1"))
			})
		})
	})

	Describe("Version", func() {
		var (
			cmd      *fakes.BOSHCommand
			executor bosh.Executor
		)
		BeforeEach(func() {
			cmd = &fakes.BOSHCommand{}
			cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
				stdout.Write([]byte("some-text version 2.0.24 some-other-text"))
				return nil
			}

			executor = bosh.NewExecutor(cmd, ioutil.ReadFile, json.Unmarshal, json.Marshal, ioutil.WriteFile)
		})

		It("passes the correct args and dir to run command", func() {
			_, err := executor.Version()
			Expect(err).NotTo(HaveOccurred())

			_, _, args := cmd.RunArgsForCall(0)
			Expect(args).To(Equal([]string{"-v"}))
		})

		It("returns the correctly trimmed version", func() {
			version, err := executor.Version()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("2.0.24"))
		})

		Context("failure cases", func() {
			Context("when the run cmd fails", func() {
				BeforeEach(func() {
					cmd.RunReturns(errors.New("failed to run cmd"))
				})

				It("returns an error", func() {
					_, err := executor.Version()
					Expect(err).To(MatchError("failed to run cmd"))
				})
			})

			Context("when the version cannot be parsed", func() {
				var expectedError error

				BeforeEach(func() {
					expectedError = bosh.NewBOSHVersionError(errors.New("BOSH version could not be parsed"))
					cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
						stdout.Write([]byte(""))
						return nil
					}
				})

				It("returns a bosh version error", func() {
					_, err := executor.Version()
					Expect(err).To(Equal(expectedError))
				})
			})
		})
	})
})

func formatScript(command string, stateDir string, args []string) string {
	script := fmt.Sprintf("#!/bin/sh\nbosh-path %s \\\n", command)
	for _, arg := range args {
		if arg[0] == '-' {
			script = fmt.Sprintf("%s  %s", script, arg)
		} else {
			script = fmt.Sprintf("%s  %s \\\n", script, arg)
		}
	}

	return fmt.Sprintf("%s\n", script[:len(script)-2])
}
