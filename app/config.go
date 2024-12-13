package app

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dan13ram/wpokt-validator/models"
	"gopkg.in/yaml.v2"
)

var (
	Config models.Config
)

func InitConfig(configFile string, envFile string) {
	log.Debug("[CONFIG] Initializing config")
	readConfigFromConfigFile(configFile)
	readConfigFromENV(envFile)
	readKeysFromGSM()
	validateConfig()
	log.Info("[CONFIG] Config initialized")
}

func readConfigFromConfigFile(configFile string) bool {
	if configFile == "" {
		log.Debug("[CONFIG] No config file provided")
		return false
	}
	log.Debugf("[CONFIG] Reading config file %s", configFile)
	var yamlFile, err = os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("[CONFIG] Error reading config file %q: %s\n", configFile, err.Error())
	}
	err = yaml.Unmarshal(yamlFile, &Config)
	if err != nil {
		log.Fatalf("[CONFIG] Error unmarshalling config file %q: %s\n", configFile, err.Error())
	}
	log.Debugf("[CONFIG] Config loaded from %s", configFile)
	return true
}

func validateConfig() {
	log.Debug("[CONFIG] Validating config")
	// mongodb
	if Config.MongoDB.URI == "" {
		log.Fatal("[CONFIG] MongoDB.URI is required")
	}
	if Config.MongoDB.Database == "" {
		log.Fatal("[CONFIG] MongoDB.Database is required")
	}
	if Config.MongoDB.TimeoutMillis == 0 {
		log.Fatal("[CONFIG] MongoDB.TimeoutMillis is required")
	}

	// ethereum
	if Config.Ethereum.RPCURL == "" {
		log.Fatal("[CONFIG] Ethereum.RPCURL is required")
	}
	if Config.Ethereum.ChainId == "" {
		log.Fatal("[CONFIG] Ethereum.ChainId is required")
	}
	if Config.Ethereum.RPCTimeoutMillis == 0 {
		log.Fatal("[CONFIG] Ethereum.RPCTimeoutMillis is required")
	}
	if Config.Ethereum.PrivateKey == "" {
		log.Fatal("[CONFIG] Ethereum.PrivateKey is required")
	}
	if strings.HasPrefix(Config.Ethereum.PrivateKey, "0x") {
		Config.Ethereum.PrivateKey = Config.Ethereum.PrivateKey[2:]
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
	if Config.Pocket.RPCTimeoutMillis == 0 {
		log.Fatal("[CONFIG] Pocket.RPCTimeoutMillis is required")
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

	// services
	if Config.MintMonitor.Enabled && Config.MintMonitor.IntervalMillis == 0 {
		log.Fatal("[CONFIG] MintMonitor.Interval is required")
	}
	if Config.MintSigner.Enabled && Config.MintSigner.IntervalMillis == 0 {
		log.Fatal("[CONFIG] MintSigner.Interval is required")
	}
	if Config.MintExecutor.Enabled && Config.MintExecutor.IntervalMillis == 0 {
		log.Fatal("[CONFIG] MintExecutor.Interval is required")
	}
	if Config.BurnMonitor.Enabled && Config.BurnMonitor.IntervalMillis == 0 {
		log.Fatal("[CONFIG] BurnMonitor.Interval is required")
	}
	if Config.BurnSigner.Enabled && Config.BurnSigner.IntervalMillis == 0 {
		log.Fatal("[CONFIG] BurnSigner.Interval is required")
	}
	if Config.BurnExecutor.Enabled && Config.BurnExecutor.IntervalMillis == 0 {
		log.Fatal("[CONFIG] BurnExecutor.Interval is required")
	}

	if Config.HealthCheck.IntervalMillis == 0 {
		log.Fatal("[CONFIG] HealthCheck.Interval is required")
	}

	log.Debug("[CONFIG] Config validated")
}
