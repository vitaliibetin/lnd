package main

// Start All LND
// Connect 1 -> 2
// OpenChannel 1 -> 2
// Stop first LND
// Connect 2 -> 3
// OpenChannel 2 -> 3
// Start first LND
// RoutinTable of first node MUST have channel 2 -> 3 
// Stop second LND
// Connect 1 -> 3
// OpenChannel 1 -> 3
// Start second LND 
// RoutingTable of second node MUST have channel 1 -> 3
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
	"strconv"
)

type LndNodeDesc struct {
	WorkDir  		string
	Host 	 		string
	PeerPort 		int
	RpcPort  		int
	LightningId 	string
	IdentityAddress string
	LndNumberToPeer map[int]int32
}

var DefaultLndNodes []LndNodeDesc = []LndNodeDesc{
	{
		WorkDir: "lnd-node1",
		Host: 	 "127.0.0.1",
		PeerPort: 10011,
		RpcPort:  10009,
		LndNumberToPeer: make(map[int]int32),
	},
	{
		WorkDir: "lnd-node2",
		Host: 	 "127.0.0.1",
		PeerPort: 11011,
		RpcPort:  11009,
		LndNumberToPeer: make(map[int]int32),
	},
	{
		WorkDir: "lnd-node3",
		Host: 	 "127.0.0.1",
		PeerPort: 12011,
		RpcPort:  12009,
		LndNumberToPeer: make(map[int]int32),
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
	lndNodesCmds []*exec.Cmd
}

func NewSimNet() *SimNet{
	return &SimNet{
		SeedDir: 	  "./simnet",
		lndNodesDesc: DefaultLndNodes,
		lndNodesCmds: make([]*exec.Cmd, len(DefaultLndNodes)),
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
	simnet.lndNodesCmds[n] = cmd
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

func (simnet *SimNet) Connect(from, to int) int32 {
	// log.Print(simnet.lndNodesDesc[from-1].RpcAddress())
	// log.Print(simnet.lndNodesDesc[to-1].ConnectionAddress())
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[from-1].RpcAddress(), 
		"connect", simnet.lndNodesDesc[to-1].ConnectionAddress())
	result, err := cmd.Output()
	if err != nil {
		log.Fatalf("Connect lnd-node%v to lnd-node%v: FAILED, Err: %v", from, to, err)
	}
	time.Sleep(time.Second)
	log.Printf("Connect lnd-node %v to lnd-node %v: SUCCESS", from, to)

	var info struct {
		PeerId int32 `json:"peer_id"`
	}
	err = json.Unmarshal([]byte(result), &info)
	if err != nil {
		log.Fatalf("Unmarshal PeerId: FAILED, Err: %v", err)
	}
	log.Printf("Unmarshal PeerId: SUCCESS, PeerId: %v", info.PeerId)
	simnet.lndNodesDesc[from-1].LndNumberToPeer[to-1] = info.PeerId
	return info.PeerId
}

func (simnet *SimNet) OpenChannel(from, to int) {
	PeerId := simnet.lndNodesDesc[from-1].LndNumberToPeer[to-1]
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[from-1].RpcAddress(), 
		"openchannel", "--peer_id", strconv.Itoa(int(PeerId)), "--local_amt", "100000", "--num_confs", "1")
	err := cmd.Run()
	if err != nil {
		log.Fatalf("OpenChannel(%v -> %v): FAILED, Err: %v", from, to, err)
	}
	time.Sleep(time.Second)
	log.Printf("OpenChannel(%v -> %v): SUCCESS", from, to)
} 

func (simnet *SimNet) ShowRoutingTable(n int) int {
	cmd := exec.Command("lncli", "--rpcserver", simnet.lndNodesDesc[n-1].RpcAddress(), "showroutingtable")
	result, err := cmd.Output()
	if err != nil {
		log.Fatalf("ShowRoutingTable: FAILED, Err: %v", err)
	}
	log.Print("ShowRoutingTable: SUCCESS")

	type ChannelDesc struct {
		ID1 string `json:"lightning_id1"`
		ID2 string `json:"lightning_id2"`
		Capacity float64 `json:"capacity"`
		Weight float64 `json:"weight"`
	}
	var info struct{
		Channels []ChannelDesc `json:"channels"`
	}
	err = json.Unmarshal(result, &info)
	if err != nil {
		log.Fatalf("Unmarshal RoutingTable: FAILED, Err: %v", err)
	}
	log.Print("Unmarshal RoutingTable: SUCCESS")
	return len(info.Channels)
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

func main() {
	simnet := NewSimNet()
	simnet.CopyEnvironment()

	simnet.StartUpAll()
	simnet.AllLndsGetInfo()

	simnet.Connect(1, 2)
	simnet.OpenChannel(1, 2)

	simnet.KillAllLnds()
	simnet.LaunchLND(2 - 1)
	simnet.LaunchLND(3 - 1)
	time.Sleep(time.Second * 6)
	simnet.OpenChannel(2, 3)
	simnet.LaunchLND(1 - 1)
	time.Sleep(time.Second * 3)
	Len := simnet.ShowRoutingTable(1)
	if Len != 2 {
		log.Fatalf("expected: %v, actual: %v", 2, Len)
	}
	log.Printf("expected: %v, actual: %v, SUCCESS", 2, 2)

	simnet.ShutdownAll()
	simnet.RemoveTemporaryEnvironment()
	log.Print("DONE!!!")
}























