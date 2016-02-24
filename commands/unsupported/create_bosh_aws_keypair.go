package unsupported

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/pivotal-cf-experimental/bosh-bootloader/aws"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/ec2"
	"github.com/pivotal-cf-experimental/bosh-bootloader/commands"
	"github.com/pivotal-cf-experimental/bosh-bootloader/state"
)

type keypairRetriever interface {
	Retrieve(session ec2.Session, name string) (ec2.KeyPairInfo, error)
}

type keypairGenerator interface {
	Generate() (ec2.Keypair, error)
}

type keypairUploader interface {
	Upload(ec2.Session, ec2.Keypair) error
}

type stateStore interface {
	Set(directory string, s state.State) error
	Get(director string) (state.State, error)
}

type sessionProvider interface {
	Session(aws.Config) (ec2.Session, error)
}

type CreateBoshAWSKeypair struct {
	retriever keypairRetriever
	generator keypairGenerator
	uploader  keypairUploader
	provider  sessionProvider
	store     stateStore
}

func NewCreateBoshAWSKeypair(retriever keypairRetriever, generator keypairGenerator, uploader keypairUploader, provider sessionProvider, store stateStore) CreateBoshAWSKeypair {
	return CreateBoshAWSKeypair{
		retriever: retriever,
		generator: generator,
		uploader:  uploader,
		provider:  provider,
		store:     store,
	}
}

func (c CreateBoshAWSKeypair) Execute(globalFlags commands.GlobalFlags) error {
	s, err := c.store.Get(globalFlags.StateDir)
	if err != nil {
		return err
	}

	session, err := c.provider.Session(aws.Config{
		AccessKeyID:      s.AWS.AccessKeyID,
		SecretAccessKey:  s.AWS.SecretAccessKey,
		Region:           s.AWS.Region,
		EndpointOverride: globalFlags.EndpointOverride,
	})
	if err != nil {
		return err
	}

	if s.KeyPair != nil {
		keyInfo, err := c.retriever.Retrieve(session, s.KeyPair.Name)
		if err != nil {
			if err != ec2.KeyPairNotFound {
				return err
			}

			return c.uploader.Upload(session, ec2.Keypair{
				Name:      s.KeyPair.Name,
				PublicKey: []byte(s.KeyPair.PublicKey),
			})
		}

		fingerprintMatches, err := verifyFingerprint(keyInfo.Fingerprint, []byte(s.KeyPair.PrivateKey))
		if err != nil {
			return err
		}

		if fingerprintMatches {
			return nil
		} else {
			return errors.New("the local keypair fingerprint does not match the keypair fingerprint on AWS, please open an issue at https://github.com/pivotal-cf-experimental/bosh-bootloader/issues/new if you require assistance.")
		}
	}

	keypair, err := c.generateAndUploadKeypair(session)
	if err != nil {
		return err
	}

	s.KeyPair = &state.KeyPair{
		Name:       keypair.Name,
		PrivateKey: string(keypair.PrivateKey),
		PublicKey:  string(keypair.PublicKey),
	}

	if err = c.store.Set(globalFlags.StateDir, s); err != nil {
		return err
	}

	return nil
}

func verifyFingerprint(awsFingerprint string, privateKeyPem []byte) (bool, error) {
	pem, _ := pem.Decode(privateKeyPem)
	if pem == nil {
		return false, errors.New("the local keypair does not contain a valid PEM encoded private key, please open an issue at https://github.com/pivotal-cf-experimental/bosh-bootloader/issues/new if you require assistance.")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(pem.Bytes)
	if err != nil {
		return false, errors.New("the local keypair does not contain a valid rsa private key, please open an issue at https://github.com/pivotal-cf-experimental/bosh-bootloader/issues/new if you require assistance.")
	}

	pkeyBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return false, err
	}

	var fingerprint string
	for _, c := range md5.Sum(pkeyBytes) {
		fingerprint += fmt.Sprintf(":%02x", c)
	}

	fingerprint = strings.TrimPrefix(fingerprint, ":")

	if awsFingerprint != fingerprint {
		return false, nil
	}

	return true, nil
}

func (c CreateBoshAWSKeypair) generateAndUploadKeypair(session ec2.Session) (ec2.Keypair, error) {
	keypair, err := c.generator.Generate()
	if err != nil {
		return ec2.Keypair{}, err
	}

	err = c.uploader.Upload(session, keypair)
	if err != nil {
		return ec2.Keypair{}, err
	}

	return keypair, nil
}