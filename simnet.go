package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/btcsuite/seelog"
	"github.com/roasbeef/btcutil"
	"golang.org/x/net/context"
)

type TopologyConfig struct {
	WalletBalances map[string]btcutil.Amount `json:"wallet_balances"`
	Channels       []Channel                 `json:"channels"`
	Connections    []Connection              `json:"connections"`
}

type Channel struct {
	Name1    string         `json:"name1"`
	Name2    string         `json:"name2"`
	Capacity btcutil.Amount `json:"capacity"`
	PushAmt  btcutil.Amount `json:"push_amt"`
}

type Connection struct {
	Name1 string `json:"name1"`
	Name2 string `json:"name2"`
}

func (conn *Connection) Normalize() {
	if conn.Name1 > conn.Name2 {
		conn.Name1, conn.Name2 = conn.Name2, conn.Name1
	}
}

func loadTopologyConfig(cfgPath string) (*TopologyConfig, error) {
	log.Debugf("Config will load from %v\n", cfgPath)
	buffer, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfg TopologyConfig
	if err := json.Unmarshal(buffer, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type TopologyManager struct {
	net       *networkHarness
	nameToLnd map[string]*lightningNode
}

func NewTopologyManager(net *networkHarness) *TopologyManager {
	return &TopologyManager{
		net:       net,
		nameToLnd: make(map[string]*lightningNode),
	}
}

func (manager *TopologyManager) ApplyFromFile(cfgPath string) error {
	cfg, err := loadTopologyConfig(cfgPath)
	if err != nil {
		return err
	}
	if err := manager.ConfigureAndVerify(cfg); err != nil {
		return err
	}
	return nil
}

// 1. Validate input
//   1.1 Check starting balances
//   1.2 Create list of needed connections
// 2. Start new lnds
// 3. ConfigureWalletBalance
// 4. ConfigureConnections
// 5. ConfigureChannelBalance
func (manager *TopologyManager) ConfigureAndVerify(cfg *TopologyConfig) error {
	if err := verifyParams(cfg.WalletBalances, cfg.Channels); err != nil {
		return err
	}
	for name, _ := range cfg.WalletBalances {
		lnd, err := manager.net.NewNode(nil)
		if err != nil {
			return err
		}
		manager.nameToLnd[name] = lnd
	}
	if err := manager.ConfigureWalletBalance(cfg.WalletBalances); err != nil {
		return err
	}
	if err := manager.ConfigureConnections(mergeSlices(cfg.Connections, cfg.Channels)); err != nil {
		return err
	}
	if err := manager.ConfigureChannelBalance(cfg.Channels); err != nil {
		return err
	}
	return nil
}

func (manager *TopologyManager) ConfigureWalletBalance(nameToAmount map[string]btcutil.Amount) error {
	for name, amount := range nameToAmount {
		if amount > 0 {
			ctx := context.Background()
			lnd := manager.nameToLnd[name]
			log.Debugf("sending coins to %v", name)
			if err := manager.net.SendCoins(ctx, amount, lnd); err != nil {
				return err
			}
			log.Debugf("coins were successfully sent to %v", name)
		}
	}
	return nil
}

func (manager *TopologyManager) ConfigureConnections(connections []Connection) error {
	for _, connection := range connections {
		ctx := context.Background()
		lnd1 := manager.nameToLnd[connection.Name1]
		lnd2 := manager.nameToLnd[connection.Name2]
		log.Debugf("trying to connect %v->%v", connection.Name1, connection.Name2)
		if err := manager.net.ConnectNodes(ctx, lnd1, lnd2); err != nil {
			return err
		}
		log.Debugf("nodes %v, %v were successfully connected", connection.Name1, connection.Name2)
	}
	return nil
}

func (manager *TopologyManager) ConfigureChannelBalance(channels []Channel) error {
	for _, channel := range channels {
		ctx := context.Background()
		lnd1 := manager.nameToLnd[channel.Name1]
		lnd2 := manager.nameToLnd[channel.Name2]
		numConfs := uint32(1)
		log.Debugf("opening channel between %v, %v", channel.Name1, channel.Name2)
		if _, err := manager.net.OpenChannel(ctx, lnd1, lnd2, channel.Capacity, channel.PushAmt, numConfs); err != nil {
			return err
		}
		log.Debugf("channel %v->%v was successfully opened", channel.Name1, channel.Name2)
	}
	return nil
}

// Ensures that bitcoin balances are larger than required lightning balances
func verifyParams(nameToAmount map[string]btcutil.Amount, channels []Channel) error {
	log.Debug("verifying configuration...")
	lightningBalances := make(map[string]btcutil.Amount, 0)
	for _, channel := range channels {
		lightningBalances[channel.Name1] += channel.Capacity
	}
	for name, lightningBalance := range lightningBalances {
		if lightningBalance > nameToAmount[name] {
			return fmt.Errorf("Requered lightning balance %v is larger than bitcoin balance %v for index %v",
				lightningBalance, nameToAmount[name], name)
		}
	}
	log.Debug("given configuration was successfully verified")
	return nil
}

func mergeSlices(connections []Connection, channels []Channel) []Connection {
	for _, channel := range channels {
		connections = append(connections,
			Connection{
				Name1: channel.Name1,
				Name2: channel.Name2,
			})
	}
	return removeDuplicates(connections)
}

func removeDuplicates(elements []Connection) []Connection {
	// Use map to record duplicates as we find them.
	encountered := map[Connection]bool{}
	result := []Connection{}

	for _, element := range elements {
		element.Normalize()

		if encountered[element] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[element] = true
			// Append to result slice.
			result = append(result, element)
		}
	}
	// Return the new slice.
	return result
}
