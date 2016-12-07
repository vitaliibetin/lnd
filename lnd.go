package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	proxy "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/lightningnetwork/lnd/chainntnfs/btcdnotify"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/btcwallet"

	"github.com/roasbeef/btcrpcclient"
	"time"
	"strings"
	"github.com/dgrijalva/jwt-go"
)

var (
	cfg             *config
	shutdownChannel = make(chan struct{})
)

type respWriterHelper struct {
	Code int
	w http.ResponseWriter
}

func newRespWriterHelper(w http.ResponseWriter) *respWriterHelper {
	return &respWriterHelper{
		Code: 200,
		w: w,
	}
}
func (w *respWriterHelper) Header() http.Header { return w.w.Header()}
func (w *respWriterHelper) Write(b []byte) (int, error) { return w.w.Write(b)}
func (w *respWriterHelper) WriteHeader(c int) {
	w.Code = c
	w.w.WriteHeader(c)
}

func logMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		respWrap := newRespWriterHelper(resp)
		start := time.Now()
		h.ServeHTTP(respWrap, req)
		duration := time.Since(start)
		rpcsLog.Infof("HTTP %v %v - %v %v %v", req.Method, req.RequestURI, respWrap.Code, http.StatusText(respWrap.Code), duration)
	})
}

func authMiddleware(h http.Handler, secret []byte) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		authStr := req.Header.Get("Authorization")
		// Check that it has form Bearer <token>
		split := strings.Split(authStr, " ")
		if len(split) == 2 && split[0] == "Bearer" {
			_, err := jwt.Parse(split[1], func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unsuported signature type")
				}
				return secret, nil
			})
			if err == nil {
				h.ServeHTTP(resp, req)
				return
			}
		}
		// So some error occured
		resp.WriteHeader(403)
		resp.Write([]byte(`{"error": "Access Denied"}`))
	})
}

// lndMain is the true entry point for lnd. This function is required since
// defers created in the top-level scope of a main method aren't executed if
// os.Exit() is called.
func lndMain() error {
	// Load the configuration, and parse any command line options. This
	// function will also set up logging properly.
	loadedConfig, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = loadedConfig
	defer backendLog.Flush()

	// Show version at startup.
	ltndLog.Infof("Version %s", version())

	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			fmt.Println(http.ListenAndServe(listenAddr, nil))
		}()
	}

	// Open the channeldb, which is dedicated to storing channel, and
	// network related meta-data.
	chanDB, err := channeldb.Open(cfg.DataDir, activeNetParams.Params)
	if err != nil {
		fmt.Println("unable to open channeldb: ", err)
		return err
	}
	defer chanDB.Close()

	// Next load btcd's TLS cert for the RPC connection. If a raw cert was
	// specified in the config, then we'll set that directly. Otherwise, we
	// attempt to read the cert from the path specified in the config.
	var rpcCert []byte
	if cfg.RawRPCCert != "" {
		rpcCert, err = hex.DecodeString(cfg.RawRPCCert)
		if err != nil {
			return err
		}
	} else {
		certFile, err := os.Open(cfg.RPCCert)
		if err != nil {
			return err
		}
		rpcCert, err = ioutil.ReadAll(certFile)
		if err != nil {
			return err
		}
		if err := certFile.Close(); err != nil {
			return err
		}
	}

	rpcIP, err := net.LookupHost(cfg.RPCHost)
	if err != nil {
		fmt.Printf("unable to resolve rpc host: %v", err)
		return err
	}

	btcdHost := fmt.Sprintf("%v:%v", cfg.RPCHost, activeNetParams.rpcPort)
	btcdUser := cfg.RPCUser
	btcdPass := cfg.RPCPass

	// TODO(roasbeef): parse config here and select chosen notifier instead
	rpcConfig := &btcrpcclient.ConnConfig{
		Host:                 btcdHost,
		Endpoint:             "ws",
		User:                 btcdUser,
		Pass:                 btcdPass,
		Certificates:         rpcCert,
		DisableTLS:           false,
		DisableConnectOnNew:  true,
		DisableAutoReconnect: false,
	}
	notifier, err := btcdnotify.New(rpcConfig)
	if err != nil {
		return err
	}

	// TODO(roasbeef): parse config here select chosen WalletController
	walletConfig := &btcwallet.Config{
		PrivatePass: []byte("hello"),
		DataDir:     filepath.Join(cfg.DataDir, "lnwallet"),
		RpcHost:     fmt.Sprintf("%v:%v", rpcIP[0], activeNetParams.rpcPort),
		RpcUser:     cfg.RPCUser,
		RpcPass:     cfg.RPCPass,
		CACert:      rpcCert,
		NetParams:   activeNetParams.Params,
	}
	wc, err := btcwallet.New(walletConfig)
	if err != nil {
		fmt.Printf("unable to create wallet controller: %v\n", err)
		return err
	}
	signer := wc
	bio := wc

	// Create, and start the lnwallet, which handles the core payment
	// channel logic, and exposes control via proxy state machines.
	wallet, err := lnwallet.NewLightningWallet(chanDB, notifier,
		wc, signer, bio, activeNetParams.Params)
	if err != nil {
		fmt.Printf("unable to create wallet: %v\n", err)
		return err
	}
	if err := wallet.Startup(); err != nil {
		fmt.Printf("unable to start wallet: %v\n", err)
		return err
	}
	ltndLog.Info("LightningWallet opened")

	// Set up the core server which will listen for incoming peer
	// connections.
	defaultListenAddrs := []string{
		net.JoinHostPort("", strconv.Itoa(cfg.PeerPort)),
	}
	server, err := newServer(defaultListenAddrs, notifier, bio, wallet, chanDB)
	if err != nil {
		srvrLog.Errorf("unable to create server: %v\n", err)
		return err
	}
	if err := server.Start(); err != nil {
		srvrLog.Errorf("unable to create to start: %v\n", err)
		return err
	}

	addInterruptHandler(func() {
		ltndLog.Infof("Gracefully shutting down the server...")
		server.Stop()
		server.WaitForShutdown()
	})

	// Initialize, and register our implementation of the gRPC server.
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	lnrpc.RegisterLightningServer(grpcServer, server.rpcServer)

	// Next, Start the grpc server listening for HTTP/2 connections.
	grpcEndpoint := fmt.Sprintf("0.0.0.0:%d", loadedConfig.RPCPort)
	lis, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
		return err
	}
	defer lis.Close()
	go func() {
		rpcsLog.Infof("RPC server listening on %s", lis.Addr())
		grpcServer.Serve(lis)
	}()

	// Finally, start the REST proxy for our gRPC server above.
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mux := proxy.NewServeMux()
	swaggerPattern := proxy.MustPattern(proxy.NewPattern(1, []int{2, 0, 2, 1}, []string{"v1", "swagger"}, ""))
	// TODO(roasbeef): accept path to swagger file as command-line option
	mux.Handle("GET", swaggerPattern, func(w http.ResponseWriter, r *http.Request, p map[string]string) {
		http.ServeFile(w, r, loadedConfig.SwaggerPath)
	})
	proxyOpts := []grpc.DialOption{grpc.WithInsecure()}
	err = lnrpc.RegisterLightningHandlerFromEndpoint(ctx, mux, grpcEndpoint, proxyOpts)
	if err != nil {
		return err
	}
	allowCORSMiddleware := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request){
			if req.Method == "OPTIONS" {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
				resp.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
				resp.Header().Set("Access-Control-Allow-Headers", "Authorization")
			} else {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
				h.ServeHTTP(resp, req)
			}
		})
	}
	var handler http.Handler
	if loadedConfig.HTTPSecret == "" {
		handler = mux
	} else {
		handler = authMiddleware(mux, []byte(loadedConfig.HTTPSecret))
	}
	handler = logMiddleware(allowCORSMiddleware(handler))
	go func() {
		rpcsLog.Infof("gRPC proxy started")
		http.ListenAndServe(loadedConfig.HTTPRPCAddr, handler)
	}()

	// Wait for shutdown signal from either a graceful server stop or from
	// the interrupt handler.
	<-shutdownChannel
	ltndLog.Info("Shutdown complete")
	return nil
}

func main() {
	// Use all processor cores.
	// TODO(roasbeef): remove this if required version # is > 1.6?
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	if err := lndMain(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
