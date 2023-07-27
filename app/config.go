package app

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"

	"github.com/dan13ram/wpokt-validator/models"
	"gopkg.in/yaml.v2"
)

var (
	Config models.Config
)

func InitConfig(configFile string, envFile string) {
	readConfigFromConfigFile(configFile)
	readConfigFromENV(envFile)
	readKeysFromGSM()
	validateConfig()
}

func readConfigFromConfigFile(configFile string) {
	if configFile == "" {
		return
	}
	var yamlFile, err = ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("[CONFIG] Error reading config file %q: %s\n", configFile, err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &Config)
	if err != nil {
		log.Fatalf("[CONFIG] Error unmarshalling config file %q: %s\n", configFile, err.Error())
	}
}

func validateConfig() {
	// mongodb
	if Config.MongoDB.URI == "" {
		log.Fatal("[CONFIG] MongoDB.URI is required")
	}
	if Config.MongoDB.Database == "" {
		log.Fatal("[CONFIG] MongoDB.Database is required")
	}

	// ethereum
	if Config.Ethereum.RPCURL == "" {
		log.Fatal("[CONFIG] Ethereum.RPCURL is required")
	}
	if Config.Ethereum.ChainId == "" {
		log.Fatal("[CONFIG] Ethereum.ChainId is required")
	}
	if Config.Ethereum.PrivateKey == "" {
		log.Fatal("[CONFIG] Ethereum.PrivateKey is required")
	}
	if Config.Ethereum.WrappedPocketAddress == "" {
		log.Fatal("[CONFIG] Ethereum.WrappedPocketAddress is required")
	}
	if Config.Ethereum.MintControllerAddress == "" {
		log.Fatal("[CONFIG] Ethereum.MintControllerAddress is required")
	}
	if Config.Ethereum.ValidatorAddresses == nil || len(Config.Ethereum.ValidatorAddresses) == 0 {
		log.Fatal("[CONFIG] Ethereum.ValidatorAddresses is required")
	}

	// pocket
	if Config.Pocket.RPCURL == "" {
		log.Fatal("[CONFIG] Pocket.RPCURL is required")
	}
	if Config.Pocket.ChainId == "" {
		log.Fatal("[CONFIG] Pocket.ChainId is required")
	}
	if Config.Pocket.PrivateKey == "" {
		log.Fatal("[CONFIG] Pocket.PrivateKey is required")
	}
	if Config.Pocket.TxFee == 0 {
		log.Fatal("[CONFIG] Pocket.TxFee is required")
	}
	if Config.Pocket.VaultAddress == "" {
		log.Fatal("[CONFIG] Pocket.VaultAddress is required")
	}
	if Config.Pocket.MultisigPublicKeys == nil || len(Config.Pocket.MultisigPublicKeys) == 0 {
		log.Fatal("[CONFIG] Pocket.MultisigPublicKeys is required")
	}
}
