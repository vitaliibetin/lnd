package main

import (
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"golang.org/x/net/context"
	"os"

	"encoding/json"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"io"
	"github.com/roasbeef/btcd/wire"
	"encoding/hex"
	"github.com/BitfuryLightning/tools/rt"
	"github.com/BitfuryLightning/tools/rt/graph"
	"github.com/lightningnetwork/lnd/lnwire"
)

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[grpc2http] %v\n", err)
	os.Exit(1)
}

func getClient(addr string) lnrpc.LightningClient {
	conn := getClientConn(addr)
	return lnrpc.NewLightningClient(conn)
}

func getClientConn(addr string) *grpc.ClientConn {
	opts := []grpc.DialOption{grpc.WithInsecure()}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		fatal(err)
	}
	return conn
}

type Grpc2HttpConverter struct {
	grpcAdrress string
	httpAddress string
}

func NewGrpc2HttpConverter(grpcAdrress, httpAddress string) *Grpc2HttpConverter {
	return &Grpc2HttpConverter{
		grpcAdrress: grpcAdrress,
		httpAddress: httpAddress,
	}
}

func writeJsonResp(resp http.ResponseWriter, data interface{}){
	resp.Header().Add("Access-Control-Allow-Origin", "*")
	resp.WriteHeader(200)
	json.NewEncoder(resp).Encode(data)
}


func writeErrorResp(resp http.ResponseWriter, code int, msg string){
	resp.Header().Add("Access-Control-Allow-Origin", "*")
	resp.WriteHeader(code)
	data := struct {
		Error string `json:"error"`
	} {
		Error: msg,
	}
	json.NewEncoder(resp).Encode(data)
	log.Print("ERROR:", msg)
}

func (g *Grpc2HttpConverter) handleGetInfo(resp http.ResponseWriter,
	req *http.Request) {
	log.Print("GetInfo")
	client := getClient(g.grpcAdrress)
	ctxb := context.Background()
	lnReq := &lnrpc.GetInfoRequest{}
	lnResp, _ := client.GetInfo(ctxb, lnReq)
	writeJsonResp(resp, lnResp)
}

func (g *Grpc2HttpConverter) handleNewAddress(resp http.ResponseWriter,
	req *http.Request) {
	inpData := struct {
		AddrType string `json:"addr_type"`
	}{
		AddrType: "p2wkh",
	}
	// TODO(mkl): error checking
	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
	}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
	}
	log.Printf("NewAddress addr_type=%v", inpData.AddrType)
	stringAddrType := inpData.AddrType

	// Map the string encoded address type, to the concrete typed address
	// type enum. An unrecognized address type will result in an error.
	var addrType lnrpc.NewAddressRequest_AddressType
	switch stringAddrType { // TODO(roasbeef): make them ints on the cli?
	case "p2wkh":
		addrType = lnrpc.NewAddressRequest_WITNESS_PUBKEY_HASH
	case "np2wkh":
		addrType = lnrpc.NewAddressRequest_NESTED_PUBKEY_HASH
	case "p2pkh":
		addrType = lnrpc.NewAddressRequest_PUBKEY_HASH
	default:
		writeErrorResp(resp, 500, fmt.Sprintf("invalid address type %v, support address type "+
			"are: p2wkh, np2wkh, p2pkh", stringAddrType))
		return
	}

	ctxb := context.Background()
	client := getClient(g.grpcAdrress)
	addr, err := client.NewAddress(ctxb, &lnrpc.NewAddressRequest{
		Type: addrType,
	})
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	writeJsonResp(resp, addr)
}

func (g *Grpc2HttpConverter)handleConnectPeer(resp http.ResponseWriter,
	req *http.Request) {
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct {
		TargetAddress string `json:"target_address"`
	}{}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("ConnectPeer target_address=%v", inpData.TargetAddress)
	targetAddress := inpData.TargetAddress
	splitAddr := strings.Split(targetAddress, "@")
	if len(splitAddr) != 2 {
		writeErrorResp(resp, 500, "target address expected in format: lnid@host:port")
		return
	}

	addr := &lnrpc.LightningAddress{
		PubKeyHash: splitAddr[0],
		Host:       splitAddr[1],
	}
	lnReq := &lnrpc.ConnectPeerRequest{addr}

	lnid, err := client.ConnectPeer(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	writeJsonResp(resp, lnid)
}

func (g *Grpc2HttpConverter) handleOpenChannel(resp http.ResponseWriter,
	req *http.Request) {
	// TODO(roasbeef): add deadline to context
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct {
		TargetPeerId int `json:"peer_id"`
		LocalFundingAmount int64 `json:"local_amt"`
		RemoteFundingAmount int64 `json:"remote_amt"`
		NumConfs            uint32 `json:"num_confs"`
		Block  bool         `json:"block"`
	} {}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("OpenChannel peer_id=%v local_amt=%v remote_amt=%v num_confs=%v block=%v",
		inpData.TargetPeerId,
		inpData.LocalFundingAmount,
		inpData.RemoteFundingAmount,
		inpData.NumConfs,
		inpData.Block,
	)
	lnReq := &lnrpc.OpenChannelRequest{
		TargetPeerId:        int32(inpData.TargetPeerId),
		LocalFundingAmount:  int64(inpData.LocalFundingAmount),
		RemoteFundingAmount: int64(inpData.RemoteFundingAmount),
		NumConfs:            uint32(inpData.NumConfs),
	}

	lnStream, err := client.OpenChannel(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}

	if !inpData.Block {
		writeJsonResp(resp, struct {}{})
		return
	}

	for {
		lnResp, err := lnStream.Recv()
		if err == io.EOF {
			writeJsonResp(resp, struct {}{})
			return
		} else if err != nil {
			writeErrorResp(resp, 500, err.Error())
			return
		}

		switch update := lnResp.Update.(type) {
		case *lnrpc.OpenStatusUpdate_ChanOpen:
			channelPoint := update.ChanOpen.ChannelPoint
			txid, err := wire.NewShaHash(channelPoint.FundingTxid)
			if err != nil {
				writeErrorResp(resp, 500, err.Error())
				return
			}

			index := channelPoint.OutputIndex
			json.NewEncoder(resp).Encode(struct {
				ChannelPoint string `json:"channel_point"`
			}{
				ChannelPoint: fmt.Sprintf("%v:%v", txid, index),
			},
			)
		}
	}

	writeJsonResp(resp, struct {}{})
}

func (g *Grpc2HttpConverter) handleListPeers(resp http.ResponseWriter,
	req *http.Request) {
	log.Print("ListPeers")
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	lnReq := &lnrpc.ListPeersRequest{}
	lnResp, err := client.ListPeers(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}

	writeJsonResp(resp, lnResp)
}

func (g *Grpc2HttpConverter) handleSendPayment(resp http.ResponseWriter,
	req *http.Request){
	client := getClient(g.grpcAdrress)

	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct {
		Dest string `json:"dest"`
		Amt int64 `json:"amt"`
		FastSend bool `json:"fast"`
	} {}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("SendPayment dest=%v amt=%v fast=%v", inpData.Dest, inpData.Amt, inpData.FastSend)

	destAddr, err := hex.DecodeString(inpData.Dest)
	if err != nil {
		writeErrorResp(resp, 500, fmt.Sprintf("Can't decode address: %v", err.Error()))
		return
	}
	// TODO(roasbeef): remove debug payment hash
	lnReq := &lnrpc.SendRequest{
		Dest:     destAddr,
		Amt:      int64(inpData.Amt),
		FastSend: inpData.FastSend,
	}

	paymentStream, err := client.SendPayment(context.Background())
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}

	if err := paymentStream.Send(lnReq); err != nil {
		//TODO(mkl): report error
		writeErrorResp(resp, 500, err.Error())
		return
	}

	lnResp, _ := paymentStream.Recv()
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}

	paymentStream.CloseSend()
	writeJsonResp(resp, lnResp)
}

func getRoutingTable(ctxb context.Context, client lnrpc.LightningClient) (*rt.RoutingTable, error) {
	req := &lnrpc.ShowRoutingTableRequest{}
	resp, err := client.ShowRoutingTable(ctxb, req)
	if err != nil {
		return nil, err
	}

	r := rt.NewRoutingTable()
	for _, channel := range resp.Channels {
		r.AddChannel(
			graph.NewID(channel.Id1),
			graph.NewID(channel.Id2),
			graph.NewEdgeID(channel.Outpoint),
			&rt.ChannelInfo{channel.Capacity, channel.Weight},
		)
	}
	return r, nil
}

func (g *Grpc2HttpConverter) handleGetRoutingTable(resp http.ResponseWriter,
	req *http.Request){
	log.Print("GetRoutingTable")
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	r, err := getRoutingTable(ctxb, client)
	if err != nil{
		writeErrorResp(resp, 500, err.Error())
		return
	}

	jsonRT := rtToJSONForm(r)
	writeJsonResp(resp, jsonRT)
}

func rtToJSONForm(r *rt.RoutingTable) interface{} {
	type ChannelDesc struct {
		ID1      string  `json:"lightning_id1"`
		ID2      string  `json:"lightning_id2"`
		EdgeId   string  `json:"outpoint"`
		Capacity int64   `json:"capacity"`
		Weight   float64 `json:"weight"`
	}
	var channels struct {
		Channels []ChannelDesc `json:"channels"`
	}
	channelsRaw := r.AllChannels()
	channels.Channels = make([]ChannelDesc, 0, len(channelsRaw))
	for _, channelRaw := range channelsRaw {
		sourceHex := hex.EncodeToString([]byte(channelRaw.Id1.String()))
		targetHex := hex.EncodeToString([]byte(channelRaw.Id2.String()))
		channels.Channels = append(channels.Channels,
			ChannelDesc{
				ID1:      sourceHex,
				ID2:      targetHex,
				EdgeId:   channelRaw.EdgeID.String(),
				Weight:   channelRaw.Info.Weight(),
				Capacity: channelRaw.Info.Capacity(),
			},
		)
	}
	return channels
}

func (g *Grpc2HttpConverter) handleSendMultihopPayment(resp http.ResponseWriter,
	req *http.Request) {
	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct{
		Amount int64 `json:"amount"`
		Path []string `json:"path"`
	}{}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Print("SendMultihopPayment amount=%v path=%v", inpData.Amount, inpData.Path)
	amount := inpData.Amount
	if amount <= 0 {
		writeErrorResp(resp, 500, fmt.Sprintf("Amount should be positive. Got %v", amount))
		return
	}
	path := make([]string, 0)
	for i:=0; i<len(inpData.Path); i++ {
		p, err := hex.DecodeString(inpData.Path[i])
		if err != nil {
			writeErrorResp(resp, 500, fmt.Sprintf("Amount should be positive. Got %v", amount))
			return
		}
		if len(p) != 32 {
			writeErrorResp(resp, 500, fmt.Sprintf("Size of LightningID should be 32 bytes, got %v", len(p)))
			return
		}
		path = append(path, string(p))
	}
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)
	lnReq := &lnrpc.MultihopPaymentRequest{
		Amount: amount,
		LightningIDs: path,
	}
	lnResp, err := client.SendMultihopPayment(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	writeJsonResp(resp, lnResp)
}

func validationStatusToStr(st lnwire.AllowHTLCStatus) string {
	switch st {
	case lnwire.AllowHTLCStatus_Allow:
		return "ALLOW"
	case lnwire.AllowHTLCStatus_Decline:
		return "DECLINE"
	case lnwire.AllowHTLCStatus_Timeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

func (g *Grpc2HttpConverter) handleFindPath(resp http.ResponseWriter,
	req *http.Request) {
	// TODO(roasbeef): add deadline to context
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	// TODO(mkl): errors
	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct{
		Destination string `json:"destination"`
		NumberOfPaths int32 `json:"maxpath"`
		Validate bool `json:"validate"`
		Amount int64 `json:"amount"`
		Timeout int32 `json:"timeout"`
	}{
		Timeout: 10,
	}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("FindPath destination=%v maxpath=%v validate=%v amount=%v timeout=%v",
		inpData.Destination,
		inpData.NumberOfPaths,
		inpData.Validate,
		inpData.Amount,
		inpData.Timeout,
	)
	destID, err := hex.DecodeString(inpData.Destination)
	if err != nil{
		writeErrorResp(resp, 500, err.Error())
		return
	}
	if len(destID) != 32 {
		writeErrorResp(resp, 500,fmt.Sprintf("Incorrect size of LightningID, got %v, want %v", len(destID), 32))
		return
	}
	lnReq := &lnrpc.FindPathRequest{
		TargetID: string(destID),
		NumberOfPaths: inpData.NumberOfPaths,
		Validate: inpData.Validate,
		Amount: inpData.Amount,
		Timeout: inpData.Timeout,
	}

	lnResp, err := client.FindPath(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	// We need to convert byte-strings into hex-encoded string
	type PathResult struct {
		Path []string `json:"path"`
		ValidationStatus string  `json:"validation_status"`
	}
	var convResp struct {
		Paths []PathResult `json:"paths"`
	}
	convResp.Paths = make([]PathResult, len(lnResp.Paths))
	for i := 0; i< len(lnResp.Paths); i++{
		convPath := make([]string, len(lnResp.Paths[i].Path))
		for j:=0; j<len(lnResp.Paths[i].Path); j++ {
			convPath[j] = hex.EncodeToString([]byte(lnResp.Paths[i].Path[j]))
		}
		convResp.Paths[i].Path = convPath
		convResp.Paths[i].ValidationStatus = validationStatusToStr(lnwire.AllowHTLCStatus(lnResp.Paths[i].ValidationStatus))
	}
	writeJsonResp(resp, convResp)
}

func (g *Grpc2HttpConverter) handlePayMultihop(resp http.ResponseWriter,
	req *http.Request) {
	// TODO(roasbeef): add deadline to context
	ctxb := context.Background()
	client := getClient(g.grpcAdrress)

	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct{
		Destination string `json:"destination"`
		NumberOfPaths int32 `json:"maxpath"`
		Amount int64 `json:"amount"`
		Timeout int32 `json:"timeout"`
	}{
		Timeout: 10,
		NumberOfPaths: 10,
	}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("PayMultihop destination=%v maxpath=%v amount=%v timeout=%v",
		inpData.Destination,
		inpData.NumberOfPaths,
		inpData.Amount,
		inpData.Timeout,
	)
	amount := inpData.Amount
	destID, err := hex.DecodeString(inpData.Destination)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	if len(destID) != 32 {
		writeErrorResp(resp, 500,fmt.Sprintf("Incorrect size of LightningID, got %v, want %v", len(destID), 32))
		return
	}
	lnReq := &lnrpc.FindPathRequest{
		TargetID: string(destID),
		NumberOfPaths: inpData.NumberOfPaths,
		Validate: true,
		Amount: amount,
		Timeout: inpData.Timeout,
	}

	lnResp, err := client.FindPath(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	iPathToUse := -1
	var convPathToUse []string
	// We need to convert byte-strings into hex-encoded string
	for i := 0; i< len(lnResp.Paths); i++{
		convPath := make([]string, len(lnResp.Paths[i].Path))
		for j:=0; j<len(lnResp.Paths[i].Path); j++ {
			convPath[j] = hex.EncodeToString([]byte(lnResp.Paths[i].Path[j]))
		}
		status := lnwire.AllowHTLCStatus(lnResp.Paths[i].ValidationStatus)
		// We need first valid path because it is shortest
		if iPathToUse == -1 && status == lnwire.AllowHTLCStatus_Allow{
			iPathToUse = i
			convPathToUse = convPath
		}
	}
	if iPathToUse == -1 {
		log.Print("We didn't find any suitable path")
		writeErrorResp(resp, 500, "No routes found")
	} else {
		pathToUse := lnResp.Paths[iPathToUse].Path
		log.Printf("Will make payment %v using path %v\n", amount, convPathToUse)
		lnReq2 := &lnrpc.MultihopPaymentRequest{
			Amount: amount,
			// We need to ignore first node(because it is ourselves)
			LightningIDs: pathToUse[1:],
		}
		_, err := client.SendMultihopPayment(ctxb, lnReq2)
		if err != nil {
			writeErrorResp(resp, 500, err.Error())
			return
		}
		log.Print("Payment send")
	}
	writeJsonResp(resp, struct {}{})
	resp.Write([]byte("{}"))
}

func (g *Grpc2HttpConverter) handleWalletBalance(resp http.ResponseWriter,
	req *http.Request) {
	client := getClient(g.grpcAdrress)
	inp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	inpData := struct{
		WitnessOnly bool `json:"witness_only"`
	}{
		WitnessOnly: false,
	}
	err = json.Unmarshal(inp, &inpData)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	log.Printf("WalletBalance witness_only=%v", inpData.WitnessOnly)
	lnReq := &lnrpc.WalletBalanceRequest{
		WitnessOnly: inpData.WitnessOnly,
	}
	ctxb := context.Background()
	lnResp, err := client.WalletBalance(ctxb, lnReq)
	if err != nil {
		writeErrorResp(resp, 500, err.Error())
		return
	}
	writeJsonResp(resp, lnResp)
}


type notFoundHandler struct{}

func (a notFoundHandler) ServeHTTP(resp http.ResponseWriter,
	req *http.Request) {
	log.Printf("Request to not existing path: %v", req.RequestURI)
	resp.Header().Add("Access-Control-Allow-Origin", "*")
	writeErrorResp(resp, 400, "Requested path not found")
}

func (g *Grpc2HttpConverter)RunServer(){
	router := mux.NewRouter()
	router.NotFoundHandler = notFoundHandler{}
	router.HandleFunc("/api/get_info", g.handleGetInfo).Methods("GET")
	router.HandleFunc("/api/new_address", g.handleNewAddress).Methods("POST")
	router.HandleFunc("/api/connect_peer", g.handleConnectPeer).Methods("POST")
	router.HandleFunc("/api/open_channel", g.handleOpenChannel).Methods("POST")
	router.HandleFunc("/api/list_peers", g.handleListPeers).Methods("GET")
	router.HandleFunc("/api/send_payment", g.handleSendPayment).Methods("POST")
	router.HandleFunc("/api/get_routing_table", g.handleGetRoutingTable).Methods("GET")
	router.HandleFunc("/api/send_multihop", g.handleSendMultihopPayment).Methods("POST")
	router.HandleFunc("/api/find_path", g.handleFindPath).Methods("POST")
	router.HandleFunc("/api/pay_multihop", g.handlePayMultihop).Methods("POST")
	router.HandleFunc("/api/wallet_balance", g.handleWalletBalance).Methods("POST")
	log.Fatal(http.ListenAndServe(g.httpAddress, router))
}

func main() {
	if len(os.Args) !=3 {
		fmt.Println("Usage: grpc2http <rpcAddress> <httpAddress>")
		os.Exit(1)
	}
	g := NewGrpc2HttpConverter(os.Args[1], os.Args[2])
	g.RunServer()
}
