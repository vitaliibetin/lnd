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
	// "fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)



type SimNet struct {
	SeedDir 	 string
	WorkDir 	 string // temporary directory
	btcdDir 	 string
	btcWalletDir string
	btcctlDir 	 string
}

func NewSimNet() *SimNet{
	return &SimNet{
		SeedDir: "./simnet",
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
		log.Fatal("Creating working directory: FAILED, Err: ", err)
	}
	log.Print("Creating working directory: SUCCESS")

	cmd := exec.Command("cp", "-r", simnet.SeedDir+"/", simnet.WorkDir+"/")
	err = cmd.Run()
	if err != nil {
		log.Fatal("Copying of environment - FAILED, Err:", err)
	}
	log.Print("Copying of environment - SUCCESS")

	simnet.btcdDir 	    = filepath.Join(simnet.btcdDir, "btcd")
	simnet.btcWalletDir = filepath.Join(simnet.btcdDir, "btcwallet")
	simnet.btcctlDir   = filepath.Join(simnet.btcdDir, "btcctl")
}

func (simnet *SimNet) RemoveTemporaryEnvironment() {
	cmd := exec.Command("rm", "-r", simnet.WorkDir)
	err := cmd.Run()
	if err != nil {
		log.Fatal("Removing of environment - FAILED, Err:", err)
	}
	log.Print("Removing of environment - SUCCESS")
}

func (simnet *SimNet) LauncnBTCD() {
	cmd := exec.Command("bash", "start-btcd.sh")
	cmd.Dir = simnet.btcdDir
	cmd.Run()
	time.Sleep(time.Second)
	// cmd.Wait()
}

func main() {
	simnet := NewSimNet()
	simnet.CopyEnvironment()

	simnet.LauncnBTCD()

	simnet.RemoveTemporaryEnvironment()
}





