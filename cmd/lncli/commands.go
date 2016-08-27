package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/roasbeef/btcd/wire"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"github.com/BitfuryLightning/tools/rt"
	"github.com/BitfuryLightning/tools/rt/graph"
	"github.com/BitfuryLightning/tools/prefix_tree"
	"github.com/BitfuryLightning/tools/rt/visualizer"
	"errors"
)

// TODO(roasbeef): cli logic for supporting both positional and unix style
// arguments.

func printRespJson(resp interface{}) {
	b, err := json.Marshal(resp)
	if err != nil {
		fatal(err)
	}

	// TODO(roasbeef): disable 'omitempty' like behavior

	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	out.WriteTo(os.Stdout)
}

var ShellCommand = cli.Command{
	Name:  "shell",
	Usage: "enter interactive shell",
	Action: func(c *cli.Context) {
		println("not implemented yet")
	},
}

var NewAddressCommand = cli.Command{
	Name:   "newaddress",
	Usage:  "generates a new address. Three address types are supported: p2wkh, np2wkh, p2pkh",
	Action: newAddress,
}

func newAddress(ctx *cli.Context) error {
	client := getClient(ctx)

	stringAddrType := ctx.Args().Get(0)

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
		return fmt.Errorf("invalid address type %v, support address type "+
			"are: p2wkh, np2wkh, p2pkh", stringAddrType)
	}

	ctxb := context.Background()
	addr, err := client.NewAddress(ctxb, &lnrpc.NewAddressRequest{
		Type: addrType,
	})
	if err != nil {
		return err
	}

	printRespJson(addr)
	return nil
}

var SendCoinsCommand = cli.Command{
	Name:        "sendcoins",
	Description: "send a specified amount of bitcoin to the passed address",
	Usage:       "sendcoins --addr=<bitcoin addresss> --amt=<num coins in satoshis>",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "addr",
			Usage: "the bitcoin address to send coins to on-chain",
		},
		// TODO(roasbeef): switch to BTC on command line? int may not be sufficient
		cli.IntFlag{
			Name:  "amt",
			Usage: "the number of bitcoin denominated in satoshis to send",
		},
	},
	Action: sendCoins,
}

func sendCoins(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.SendCoinsRequest{
		Addr:   ctx.String("addr"),
		Amount: int64(ctx.Int("amt")),
	}
	txid, err := client.SendCoins(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(txid)
	return nil
}

var SendManyCommand = cli.Command{
	Name: "sendmany",
	Description: "create and broadcast a transaction paying the specified " +
		"amount(s) to the passed address(es)",
	Usage:  `sendmany '{"ExampleAddr": NumCoinsInSatoshis, "SecondAddr": NumCoins}'`,
	Action: sendMany,
}

func sendMany(ctx *cli.Context) error {
	var amountToAddr map[string]int64

	jsonMap := ctx.Args().Get(0)
	if err := json.Unmarshal([]byte(jsonMap), &amountToAddr); err != nil {
		return err
	}

	ctxb := context.Background()
	client := getClient(ctx)

	txid, err := client.SendMany(ctxb, &lnrpc.SendManyRequest{amountToAddr})
	if err != nil {
		return err
	}

	printRespJson(txid)
	return nil
}

var ConnectCommand = cli.Command{
	Name:   "connect",
	Usage:  "connect to a remote lnd peer: <lnid>@host",
	Action: connectPeer,
}

func connectPeer(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	targetAddress := ctx.Args().Get(0)
	splitAddr := strings.Split(targetAddress, "@")
	if len(splitAddr) != 2 {
		return fmt.Errorf("target address expected in format: lnid@host:port")
	}

	addr := &lnrpc.LightningAddress{
		PubKeyHash: splitAddr[0],
		Host:       splitAddr[1],
	}
	req := &lnrpc.ConnectPeerRequest{addr}

	lnid, err := client.ConnectPeer(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(lnid)
	return nil
}

// TODO(roasbeef): default number of confirmations
var OpenChannelCommand = cli.Command{
	Name: "openchannel",
	Description: "Attempt to open a new channel to an existing peer, " +
		"blocking until the channel is 'open'. Once the channel is " +
		"open, a channelPoint (txid:vout) of the funding output is " +
		"returned.",
	Usage: "openchannel --peer_id=X --local_amt=N --remote_amt=N --num_confs=N",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "peer_id",
			Usage: "the id of the peer to open a channel with",
		},
		cli.IntFlag{
			Name:  "local_amt",
			Usage: "the number of satoshis the wallet should commit to the channel",
		},
		cli.IntFlag{
			Name:  "remote_amt",
			Usage: "the number of satoshis the remote peer should commit to the channel",
		},
		cli.IntFlag{
			Name: "num_confs",
			Usage: "the number of confirmations required before the " +
				"channel is considered 'open'",
		},
		cli.BoolFlag{
			Name:  "block",
			Usage: "block and wait until the channel is fully open",
		},
	},
	Action: openChannel,
}

func openChannel(ctx *cli.Context) error {
	// TODO(roasbeef): add deadline to context
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.OpenChannelRequest{
		TargetPeerId:        int32(ctx.Int("peer_id")),
		LocalFundingAmount:  int64(ctx.Int("local_amt")),
		RemoteFundingAmount: int64(ctx.Int("remote_amt")),
		NumConfs:            uint32(ctx.Int("num_confs")),
	}

	stream, err := client.OpenChannel(ctxb, req)
	if err != nil {
		return err
	}

	if !ctx.Bool("block") {
		return nil
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		txid, err := wire.NewShaHash(resp.ChannelPoint.FundingTxid)
		if err != nil {
			return err
		}

		index := resp.ChannelPoint.OutputIndex
		printRespJson(struct {
			ChannelPoint string `json:"channel_point"`
		}{
			ChannelPoint: fmt.Sprintf("%v:%v", txid, index),
		},
		)

	}

	return nil
}

// TODO(roasbeef): also allow short relative channel ID.
var CloseChannelCommand = cli.Command{
	Name: "closechannel",
	Description: "Close an existing channel. The channel can be closed either " +
		"cooperatively, or uncooperatively (forced).",
	Usage: "closechannel funding_txid output_index time_limit allow_force",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "funding_txid",
			Usage: "the txid of the channel's funding transaction",
		},
		cli.IntFlag{
			Name: "output_index",
			Usage: "the output index for the funding output of the funding " +
				"transaction",
		},
		cli.StringFlag{
			Name: "time_limit",
			Usage: "a relative deadline afterwhich the attempt should be " +
				"abandonded",
		},
		cli.BoolFlag{
			Name: "force",
			Usage: "after the time limit has passed, attempted an " +
				"uncooperative closure",
		},
		cli.BoolFlag{
			Name:  "block",
			Usage: "block until the channel is closed",
		},
	},
	Action: closeChannel,
}

func closeChannel(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	txid, err := wire.NewShaHashFromStr(ctx.String("funding_txid"))
	if err != nil {
		return err
	}

	req := &lnrpc.CloseChannelRequest{
		ChannelPoint: &lnrpc.ChannelPoint{
			FundingTxid: txid[:],
			OutputIndex: uint32(ctx.Int("output_index")),
		},
	}

	stream, err := client.CloseChannel(ctxb, req)
	if err != nil {
		return err
	}

	if !ctx.Bool("block") {
		return nil
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		printRespJson(resp)
	}

	return nil
}

var ListPeersCommand = cli.Command{
	Name:        "listpeers",
	Description: "List all active, currently connected peers.",
	Action:      listPeers,
}

func listPeers(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.ListPeersRequest{}
	resp, err := client.ListPeers(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(resp)
	return nil
}

var WalletBalanceCommand = cli.Command{
	Name:        "walletbalance",
	Description: "compute and display the wallet's current balance",
	Usage:       "walletbalance --witness_only=[true|false]",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "witness_only",
			Usage: "if only witness outputs should be considered when " +
				"calculating the wallet's balance",
		},
	},
	Action: walletBalance,
}

func walletBalance(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.WalletBalanceRequest{
		WitnessOnly: ctx.Bool("witness_only"),
	}
	resp, err := client.WalletBalance(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(resp)
	return nil
}

var GetInfoCommand = cli.Command{
	Name:        "getinfo",
	Description: "returns basic information related to the active daemon",
	Action:      getInfo,
}

func getInfo(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.GetInfoRequest{}
	resp, err := client.GetInfo(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(resp)
	return nil
}

var PendingChannelsCommand = cli.Command{
	Name:        "pendingchannels",
	Description: "display information pertaining to pending channels",
	Usage:       "pendingchannels --status=[all|opening|closing]",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "open, o",
			Usage: "display the status of new pending channels",
		},
		cli.BoolFlag{
			Name:  "close, c",
			Usage: "display the status of channels being closed",
		},
		cli.BoolFlag{
			Name: "all, a",
			Usage: "display the status of channels in the " +
				"process of being opened or closed",
		},
	},
	Action: pendingChannels,
}

func pendingChannels(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	var channelStatus lnrpc.ChannelStatus
	switch {
	case ctx.Bool("all"):
		channelStatus = lnrpc.ChannelStatus_ALL
	case ctx.Bool("open"):
		channelStatus = lnrpc.ChannelStatus_OPENING
	case ctx.Bool("close"):
		channelStatus = lnrpc.ChannelStatus_CLOSING
	default:
		channelStatus = lnrpc.ChannelStatus_ALL
	}

	req := &lnrpc.PendingChannelRequest{channelStatus}
	resp, err := client.PendingChannels(ctxb, req)
	if err != nil {
		return err
	}

	printRespJson(resp)

	return nil
}

var SendPaymentCommand = cli.Command{
	Name:        "sendpayment",
	Description: "send a payment over lightning",
	Usage:       "sendpayment --dest=[node_id] --amt=[in_satoshis]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "dest, d",
			Usage: "lightning address of the payment recipient",
		},
		cli.IntFlag{ // TODO(roasbeef): float64?
			Name:  "amt, a",
			Usage: "number of satoshis to send",
		},
		cli.StringFlag{
			Name:  "payment_hash, r",
			Usage: "the hash to use within the payment's HTLC",
		},
		cli.BoolFlag{
			Name: "fast, f",
			Usage: "skip the HTLC trickle logic, immediately creating a " +
				"new commitment",
		},
	},
	Action: sendPaymentCommand,
}

func sendPaymentCommand(ctx *cli.Context) error {
	client := getClient(ctx)

	destAddr, err := hex.DecodeString(ctx.String("dest"))
	if err != nil {
		return err
	}
	// TODO(roasbeef): remove debug payment hash
	req := &lnrpc.SendRequest{
		Dest:     destAddr,
		Amt:      int64(ctx.Int("amt")),
		FastSend: ctx.Bool("fast"),
	}
	paymentStream, err := client.SendPayment(context.Background())
	if err != nil {
		return err
	}
	if err := paymentStream.Send(req); err != nil {
		return err
	}
	resp, err := paymentStream.Recv()
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	paymentStream.CloseSend()
	printRespJson(resp)
	return nil
}

var ShowRoutingTableCommand = cli.Command{
	Name:        "showroutingtable",
	Description: "shows routing table for a node",
	Usage:       "showroutingtable [--table]",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "table",
			Usage: "Show the routing table in table format. Print only a few first symbols of id",
		},
		cli.StringFlag{
			Name:  "type",
			Usage: "Type of image file. Use one of: http://www.graphviz.org/content/output-formats",
		},
		cli.StringFlag{
			Name:  "viz",
			Usage: "Specifies where to save the generated file. If don't specified use os.TempDir",
		},
		cli.BoolFlag{
			Name:  "open",
			Usage: "Open generated file automatically",
		},
	},
	Action:      showRoutingTable,
}

func showRoutingTable(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	req := &lnrpc.ShowRoutingTableRequest{}
	resp, err := client.ShowRoutingTable(ctxb, req)
	if err != nil {
		return err
	}

	req2 := &lnrpc.GetInfoRequest{}
	resp2, err := client.GetInfo(ctxb, req2)
	if err != nil {
		return err
	}
	self, _ := hex.DecodeString(resp2.LightningId)

	buff := bytes.NewBuffer([]byte(resp.Rt))
	r, err := rt.UnmarshallRoutingTable(buff)
	if err != nil {
		fmt.Println("Can't unmarshall routing table")
		return err
	}

	typ := ctx.String("type")
	viz := ctx.String("viz")
	if typ != "" || viz != "" {
		TempFile, err  := ioutil.TempFile("", "") 
		if err != nil {
			return err
		}
		var ImageFile *os.File
		// if the type is not specified explicitly parse the filename
		if typ == "" {
			typ = filepath.Ext(viz)[1:]
		} 
		// if the filename is not specified explicitly use tempfile
		if viz == "" {
			ImageFile, err = TempFileWithSuffix("", "rt_", "."+typ)
			if err != nil {
				return err
			}
		} else {
			ImageFile, err = os.Create(viz)
			if err != nil {
				return err
			}
		}
		if _, ok := visualizer.SupportedFormatsAsMap()[typ]; !ok {
			fmt.Printf("Format: '%v' not recognized. Use one of: %v\n", typ, visualizer.SupportedFormats())
			return nil
		}
		// generate description graph by dot language
		writeToTempFile(r, TempFile, self)
		writeToImageFile(TempFile, ImageFile)
		if ctx.Bool("open") {
			if err := visualizer.Open(ImageFile); err != nil {
				return err
			}
		}
		return nil
	}

	if ctx.Bool("table") {
		printRTAsTable(r)
	} else {
		printRTAsJSON(r)
	}
	return nil
}

func writeToTempFile(r *rt.RoutingTable, file *os.File, self []byte) {
	slc := []graph.ID{graph.NewID(string(self))}
	viz := visualizer.New(r.G, slc, nil, nil)
	viz.ApplyToNode = func(s string) string { return hex.EncodeToString([]byte(s)) }
	viz.ApplyToEdge = func(info interface{}) string { 
		if info, ok := info.(*rt.ChannelInfo); ok {
			return fmt.Sprintf(`"%v"`, info.Capacity())
		}
		return "nil"
	}
	// need to call method if plan to use shortcut, autocomplete, etc
	viz.BuildPrefixTree()
	viz.EnableShortcut(true)
	dot := viz.Draw()
	file.Write([]byte(dot))
	file.Sync()
}

func writeToImageFile(TempFile, ImageFile *os.File) {
	visualizer.Run("neato", TempFile, ImageFile)
	TempFile.Close()
	os.Remove(TempFile.Name())
	ImageFile.Close()
}

// get around a bug in the standard library, add suffix param
func TempFileWithSuffix(dir, prefix, suffix string) (f *os.File, err error) {
	f, err = ioutil.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	f, err = os.Create(f.Name()+suffix)
	return
}

// Prints routing table in human readable table format
func printRTAsTable(r *rt.RoutingTable){
	tmpl := "%10v %10v %10v %10v\n"
	fmt.Printf(tmpl, "ID1", "ID2", "Capacity", "Weight")
	// Generate prefix tree for shortcuts
	tree := prefix_tree.NewPrefixTree()
	for _, node := range r.Nodes(){
		tree.Add( hex.EncodeToString([]byte(node.String())))
	}
	edges := r.G.GetUndirectedEdges()
	minLen := 6
	for _, edge := range(edges){
		info := edge.Info().(*rt.ChannelInfo)
		sourceHex := hex.EncodeToString([]byte(edge.Source().String()))
		source := getShortcut(tree, sourceHex, minLen)
		targetHex := hex.EncodeToString([]byte(edge.Target().String()))
		target := getShortcut(tree, targetHex, minLen)
		fmt.Printf(tmpl, source, target, info.Cpt, info.Wgt)
	}
}

func getShortcut(tree prefix_tree.PrefixTree, s string, minLen int) string {
	s1, err := tree.Shortcut(s)
	if err != nil || s == s1 {
		return s
	}
	if len(s1) < minLen && minLen < len(s) {
		s1 = s[:minLen]
	}
	shortcut := fmt.Sprintf("%v...", s1)
	if len(shortcut) >= len(s){
		shortcut = s
	}
	return shortcut
}

func printRTAsJSON(r *rt.RoutingTable){
	type  ChannelDesc struct {
		ID1 string `json:"lightning_id1"`
		ID2 string `json:"lightning_id2"`
		Capacity float64 `json:"capacity"`
		Weight float64 `json:"weight"`
	}
	var channels struct{
		Channels []ChannelDesc `json:"channels"`
	}
	edges := r.G.GetUndirectedEdges()
	channels.Channels = make([]ChannelDesc, 0, len(edges))
	for _, edge := range(edges){
		info := edge.Info().(*rt.ChannelInfo)
		sourceHex := hex.EncodeToString([]byte(edge.Source().String()))
		targetHex := hex.EncodeToString([]byte(edge.Target().String()))
		channels.Channels = append(channels.Channels,
			ChannelDesc{
				ID1: sourceHex,
				ID2: targetHex,
				Weight: info.Weight(),
				Capacity: info.Capacity(),
			},
		)
	}
	printRespJson(channels)
}