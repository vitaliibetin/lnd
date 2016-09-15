package main

// Note: Try it means connect by lightning id

// Launch lnd-nodes: 1, 2, 3
// DeleteNetworkInfo: delete cached network data from all lnd nodes
// Try 1 -> 2, expected: FAIL
// Try 2 -> 3, expected: FAIL
// Try 1 -> 3, expected: FAIL
// Connect 1 -> 2
// Connect 2 -> 3
// Shutdown All
// Start-up All
// Try 1 -> 2, expected: SUCCESS
// Try 2 -> 3, expected: SUCCESS
// Try 1 -> 3, expected: FAIL
// Search Network Info
// Try 1 -> 3, expected: SUCCESS
// DONE

import (
	"log"
	"io/ioutil"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"encoding/json"
)

type LndNodeDesc struct {
	WorkDir  		string
	Host 	 		string
	PeerPort 		int
	RpcPort  		int
	LightningId 	string
	IdentityAddress string
}

var DefaultLndNodes []LndNodeDesc = []LndNodeDesc{
	{
		WorkDir: "lnd-node1",
		Host: 	 "127.0.0.1",
		PeerPort: 10011,
		RpcPort:  10009,
	},
	{
		WorkDir: "lnd-node2",
		Host: 	 "127.0.0.1",
		PeerPort: 11011,
		RpcPort:  11009,
	},
	{
		WorkDir: "lnd-node3",
		Host: 	 "127.0.0.1",
		PeerPort: 12011,
		RpcPort:  12009,
	},
}

func (lnd *LndNodeDesc) ConnectionAddress() string {
	return fmt.Sprintf("%v@%v:%v", lnd.IdentityAddress, lnd.Host, lnd.PeerPort)
}

func (lnd *LndNodeDesc) RpcAddress() string {
	return fmt.Sprintf("%v:%v", lnd.Host, lnd.RpcPort)
}

type SimNet struct {
	SeedDir 	 string
	WorkDir 	 string // temporary directory
	btcdDir 	 string
	btcWalletDir string
	btcctlDir 	 string
	lndNodesDesc []LndNodeDesc
}

func NewSimNet() *SimNet{
	return &SimNet{
		SeedDir: 	  "./simnet",
		lndNodesDesc: DefaultLndNodes,
	}
}

func (simnet *SimNet) CopyEnvironment() {
	var err error
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	simnet.WorkDir, err = ioutil.TempDir(dir, "simnet")
	if err != nil {
		log.Fatalf("Creating working directory: FAILED, Err: ", err)
	}
	log.Print("Creating working directory: SUCCESS")

	cmd := exec.Command("cp", "-r", simnet.SeedDir+"/", simnet.WorkDir+"/")
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Copying of environment: FAILED, Err:", err)
	}
	log.Print("Copying of environment: SUCCESS")

	simnet.btcdDir 	    = filepath.Join(simnet.WorkDir, "btcd")
	simnet.btcWalletDir = filepath.Join(simnet.WorkDir, "btcwallet")
	simnet.btcctlDir    = filepath.Join(simnet.WorkDir, "btcctl")

	for i := 0; i < len(simnet.lndNodesDesc); i++ {
		simnet.lndNodesDesc[i].WorkDir = filepath.Join(simnet.WorkDir, simnet.lndNodesDesc[i].WorkDir)
	}
}

func (simnet *SimNet) RemoveTemporaryEnvironment() {
	cmd := exec.Command("rm", "-r", simnet.WorkDir)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Removing of environment: FAILED, Err:", err)
	}
	log.Print("Removing of environment: SUCCESS")
}

func (simnet *SimNet) LauncnBTCD() {
	cmd := exec.Command("bash", filepath.Join(simnet.btcdDir, "start-btcd.sh"))
	cmd.Dir = simnet.btcdDir
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Launching BTCD: FAILED, Err: %v", err)
	}
	time.Sleep(time.Second * 2)
	log.Print("Launching BTCD: SUCCESS")
}

func (simnet *SimNet) LauncnBTCWallet() {
	cmd := exec.Command("bash", filepath.Join(simnet.btcWalletDir, "start-btcwallet.sh"))
	cmd.Dir = simnet.btcWalletDir
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Launching BTCWallet: FAILED, Err: %v", err)
	}
	time.Sleep(time.Second * 2)
	log.Print("Launching BTCWallet: SUCCESS")
}

func (simnet *SimNet) LaunchLND(n int) {
	cmd := exec.Command("bash", filepath.Join(simnet.lndNodesDesc[n].WorkDir, "start-lnd.sh"))
	cmd.Dir = simnet.lndNodesDesc[n].WorkDir
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Launching LND: FAILED, Err: %v", err)
	}
	time.Sleep(time.Second * 1)
	log.Printf("Launching LND %v: SUCCESS", n+1)
}

func (simnet *SimNet) LaunchAllLnds() {
	for i := 0; i < len(simnet.lndNodesDesc); i++ {
		simnet.LaunchLND(i)
	}
	log.Print("Waiting: LND nodes completely started")
	time.Sleep(time.Second * 10)
	log.Print("LND nodes completely started")
}

func (simnet *SimNet) LndGetInfo(n int) {
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[n].RpcAddress(), "getinfo")
	result, err := cmd.Output()
	if err != nil {
		log.Fatalf("lncli getinfo for lnd-node%v: FAILED, Err: %v", n+1, err)
	}
	var info struct {
		LightningId string `json:"lightning_id"`
		IdentityAddress string `json:"identity_address"`
	}
	err = json.Unmarshal([]byte(result), &info)
	if err != nil {
		log.Fatalf("Unmarshal getinfo result: FAILED, Err: %v", err)
	} else {
		log.Printf("lncli getinfo for lnd-node%v: SUCCESS", n+1)
		// log.Printf("LightningId: %v", info.LightningId)
		// log.Printf("IdentityAddress: %v", info.IdentityAddress)
		simnet.lndNodesDesc[n].LightningId = info.LightningId
		simnet.lndNodesDesc[n].IdentityAddress = info.IdentityAddress
	}
}

func (simnet *SimNet) AllLndsGetInfo() {
	for i := 0; i < len(simnet.lndNodesDesc); i++ {
		simnet.LndGetInfo(i)
	}
}

func (simnet *SimNet) KillAllLnds() {
	Kill("lnd")
}

func (simnet *SimNet) KillBTCD() {
	Kill("btcd")
}

func (simnet *SimNet) KillBTCWallet() {
	Kill("btcwallet")
}

func Kill(name string) {
	cmd := exec.Command("pkill", "-9", name)
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Kill %v: FAILED Err: %v", name, err)
	}
	log.Printf("Kill %v: SUCCESS", name)
} 

func (simnet *SimNet) Connect(from, to int) {
	// log.Print(simnet.lndNodesDesc[from-1].RpcAddress())
	// log.Print(simnet.lndNodesDesc[to-1].ConnectionAddress())
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[from-1].RpcAddress(), 
		"connect", simnet.lndNodesDesc[to-1].ConnectionAddress())
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Connect lnd-node%v to lnd-node%v: FAILED, Err: %v", from, to, err)
	}
	time.Sleep(time.Second)
	log.Printf("Connect lnd-node %v to lnd-node %v: SUCCESS", from, to)
}

func (simnet *SimNet) Try(from, to int, expected bool) {
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[from-1].RpcAddress(), 
		"connect", simnet.lndNodesDesc[to-1].IdentityAddress)
	err := cmd.Run()
	if expected {
		if err != nil {
			log.Fatalf("Try lnd-node%v to lnd-node%v: FAILED, Err: %v", from, to, err)
		}
		time.Sleep(time.Second)
		// log.Printf("Try lnd-node %v to lnd-node %v: SUCCESS", from, to)	
		log.Printf("Try %v to %v expected: %v, PASSED", from, to, expected)
	} else {
		if err != nil {
			// log.Printf("Try lnd-node%v to lnd-node%v: FAILED, Err: %v", from, to, err)
			log.Printf("Try %v to %v expected: %v, PASSED", from, to, expected)
		} else {
			time.Sleep(time.Second)
			log.Fatalf("Try lnd-node %v to lnd-node %v: SUCCESS", from, to)	
		}
	}
}

func (simnet *SimNet) DeleteNetworkInfo(n int) {
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[n].RpcAddress(), "deletenetworkinfo")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("DeleteNetworkInfo: FAILED, Err: %v", err)
	}
	log.Printf("DeleteNetworkInfo: SUCCESS")
}

func (simnet *SimNet) DeleteNetworkInfoAll() {
	for i := 0; i < len(simnet.lndNodesDesc); i++ {
		simnet.DeleteNetworkInfo(i)
	}
}

func (simnet *SimNet) StartUpAll() {
	simnet.LauncnBTCD()
	simnet.LauncnBTCWallet()
	simnet.LaunchAllLnds()
}

func (simnet *SimNet) ShutdownAll() {
	simnet.KillAllLnds()
	simnet.KillBTCWallet()
	simnet.KillBTCD()
}

func (simnet *SimNet) SearchNetworkInfo(n int) {
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[n-1].RpcAddress(), "searchnetworkinfo")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("SearchNetworkInfo(%v): FAILED, Err: %v", n, err)
	}
	log.Printf("SearchNetworkInfo(%v): SUCCESS", n)
}

func main() {
	simnet := NewSimNet()
	simnet.CopyEnvironment()

	simnet.StartUpAll()

	simnet.DeleteNetworkInfoAll()
	simnet.AllLndsGetInfo()

	simnet.Try(1, 2, false)
	simnet.Try(2, 3, false)
	simnet.Try(1, 3, false)

	simnet.Connect(1, 2)
	simnet.Connect(2, 3)

	simnet.ShutdownAll()
	simnet.StartUpAll()

	simnet.Try(1, 2, true)
	simnet.Try(2, 3, true)
	simnet.Try(1, 3, false)

	simnet.SearchNetworkInfo(1)
	simnet.Try(1, 3, true)
	simnet.Try(3, 1, true)

	simnet.ShutdownAll()
	simnet.RemoveTemporaryEnvironment()
	log.Print("DONE!!!")
}





