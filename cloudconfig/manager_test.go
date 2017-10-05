package cloudconfig_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/cloudfoundry/bosh-bootloader/cloudconfig"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var (
		logger             *fakes.Logger
		cmd                *fakes.BOSHCommand
		stateStore         *fakes.StateStore
		opsGenerator       *fakes.CloudConfigOpsGenerator
		boshClientProvider *fakes.BOSHClientProvider
		boshClient         *fakes.BOSHClient
		socks5Proxy        *fakes.Socks5Proxy
		terraformManager   *fakes.TerraformManager
		sshKeyGetter       *fakes.SSHKeyGetter
		manager            cloudconfig.Manager

		tempDir       string
		incomingState storage.State

		baseCloudConfig []byte
	)

	BeforeEach(func() {
		logger = &fakes.Logger{}
		cmd = &fakes.BOSHCommand{}
		stateStore = &fakes.StateStore{}
		opsGenerator = &fakes.CloudConfigOpsGenerator{}
		boshClient = &fakes.BOSHClient{}
		boshClientProvider = &fakes.BOSHClientProvider{}
		socks5Proxy = &fakes.Socks5Proxy{}
		terraformManager = &fakes.TerraformManager{}
		sshKeyGetter = &fakes.SSHKeyGetter{}

		boshClientProvider.ClientCall.Returns.Client = boshClient

		var err error
		tempDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		stateStore.GetCloudConfigDirCall.Returns.Directory = tempDir

		cmd.RunStub = func(stdout io.Writer, workingDirectory string, args []string) error {
			stdout.Write([]byte("some-cloud-config"))
			return nil
		}

		incomingState = storage.State{
			IAAS: "gcp",
			BOSH: storage.BOSH{
				DirectorAddress:  "some-director-address",
				DirectorUsername: "some-director-username",
				DirectorPassword: "some-director-password",
			},
		}

		opsGenerator.GenerateCall.Returns.OpsYAML = "some-ops"

		baseCloudConfig, err = ioutil.ReadFile("fixtures/base-cloud-config.yml")
		Expect(err).NotTo(HaveOccurred())

		manager = cloudconfig.NewManager(logger, cmd, stateStore, opsGenerator, boshClientProvider, socks5Proxy, terraformManager, sshKeyGetter)
	})

	Describe("Generate", func() {
		It("returns a cloud config yaml provided a valid bbl state", func() {
			cloudConfigYAML, err := manager.Generate(incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(stateStore.GetCloudConfigDirCall.CallCount).To(Equal(1))

			cloudConfig, err := ioutil.ReadFile(fmt.Sprintf("%s/cloud-config.yml", tempDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(cloudConfig).To(Equal(baseCloudConfig))

			Expect(opsGenerator.GenerateCall.Receives.State).To(Equal(incomingState))

			ops, err := ioutil.ReadFile(fmt.Sprintf("%s/ops.yml", tempDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(ops)).To(Equal("some-ops"))

			Expect(cmd.RunCallCount()).To(Equal(1))
			_, workingDirectory, args := cmd.RunArgsForCall(0)
			Expect(workingDirectory).To(Equal(tempDir))
			Expect(args).To(Equal([]string{
				"interpolate", fmt.Sprintf("%s/cloud-config.yml", tempDir),
				"-o", fmt.Sprintf("%s/ops.yml", tempDir),
			}))

			Expect(cloudConfigYAML).To(Equal("some-cloud-config"))
		})

		Context("failure cases", func() {
			Context("when getting cloud config dir fails", func() {
				BeforeEach(func() {
					stateStore.GetCloudConfigDirCall.Returns.Error = errors.New("failed to create temp dir")
				})

				It("returns an error", func() {
					_, err := manager.Generate(storage.State{})
					Expect(err).To(MatchError("failed to create temp dir"))
				})
			})

			Context("when write file fails to write cloud-config.yml", func() {
				BeforeEach(func() {
					cloudconfig.SetWriteFile(func(filename string, body []byte, mode os.FileMode) error {
						if strings.Contains(filename, "cloud-config.yml") {
							return errors.New("failed to write file")
						}
						return nil
					})
				})

				AfterEach(func() {
					cloudconfig.ResetWriteFile()
				})

				It("returns an error", func() {
					_, err := manager.Generate(storage.State{})
					Expect(err).To(MatchError("failed to write file"))
				})
			})

			Context("when ops generator fails to generate", func() {
				BeforeEach(func() {
					opsGenerator.GenerateCall.Returns.Error = errors.New("failed to generate")
				})

				It("returns an error", func() {
					_, err := manager.Generate(storage.State{})
					Expect(err).To(MatchError("failed to generate"))
				})
			})

			Context("when write file fails to write ops.yml", func() {
				BeforeEach(func() {
					cloudconfig.SetWriteFile(func(filename string, body []byte, mode os.FileMode) error {
						if strings.Contains(filename, "ops.yml") {
							return errors.New("failed to write file")
						}
						return nil
					})
				})

				AfterEach(func() {
					cloudconfig.ResetWriteFile()
				})

				It("returns an error", func() {
					_, err := manager.Generate(storage.State{})
					Expect(err).To(MatchError("failed to write file"))
				})
			})

			Context("when command fails to run", func() {
				BeforeEach(func() {
					cmd.RunReturns(errors.New("failed to run"))
				})

				It("returns an error", func() {
					_, err := manager.Generate(storage.State{})
					Expect(err).To(MatchError("failed to run"))
				})
			})
		})
	})

	Describe("Update", func() {
		It("logs steps taken", func() {
			err := manager.Update(incomingState)
			Expect(err).NotTo(HaveOccurred())
			Expect(logger.StepCall.Messages).To(Equal([]string{
				"generating cloud config",
				"applying cloud config",
			}))
		})

		It("updates the bosh director with a cloud config provided a valid bbl state", func() {
			err := manager.Update(incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(boshClientProvider.ClientCall.Receives.DirectorAddress).To(Equal("some-director-address"))
			Expect(boshClientProvider.ClientCall.Receives.DirectorUsername).To(Equal("some-director-username"))
			Expect(boshClientProvider.ClientCall.Receives.DirectorPassword).To(Equal("some-director-password"))

			Expect(boshClient.UpdateCloudConfigCall.Receives.Yaml).To(Equal([]byte("some-cloud-config")))
		})

		Context("failure cases", func() {
			Context("when manager generate's command fails to run", func() {
				BeforeEach(func() {
					cmd.RunReturns(errors.New("failed to run"))
				})

				It("returns an error", func() {
					err := manager.Update(storage.State{})
					Expect(err).To(MatchError("failed to run"))
				})
			})

			Context("when bosh client fails to update cloud config", func() {
				BeforeEach(func() {
					boshClient.UpdateCloudConfigCall.Returns.Error = errors.New("failed to update")
				})

				It("returns an error", func() {
					err := manager.Update(storage.State{})
					Expect(err).To(MatchError("failed to update"))
				})
			})
		})
	})
})
