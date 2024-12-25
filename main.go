package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

var opts struct {
	Wallet       string `long:"wallet" env:"WALLET"`
	Port         string `long:"port" env:"PORT" default:"8080"`
	Host         string `long:"host" env:"HOST"`
	Database     string `long:"database" env:"DATABASE" default:"postgresql://user:password@localhost:5432/db?sslmode=disable"`
	EmailAddress string `long:"email-addr" env:"EMAIL_ADDRESS" default:"mail.hosting.reg.ru"`
	EmailPort    int    `long:"email-port" env:"EMAIL_PORT" default:"587"`
	EmailCreds   string `long:"email-creds" env:"EMAIL_CREDS" default:"support@chx.su:password"`
	CertFile     string `long:"cert-file" env:"CERT_FILE"`
	KeyFile      string `long:"key-file" env:"KEY_FILE"`
	Help         bool   `short:"h" long:"help"`
}

var help = `Server parameters:
--port             - Port on which to run application on
--host             - Hostname, should be inintialized on production runs
--database         - database connection string
--email-addr       - email client address
--email-port       - email client port
--email-creds      - email "login:password"
--cert-file        - Cert file path (should be used for TLS)
--key-file         - Key file path (should be used for TLS)
-h --help          - Show this help message and exit`

func main() {
	_, err := flags.NewParser(&opts, flags.IgnoreUnknown).Parse()
	if err != nil {
		panic(err)
	}
	if opts.Help {
		fmt.Println(help)
		return
	}

	client := liteclient.NewConnectionPool()

	// // get config
	cfg, err := liteclient.GetConfigFromUrl(context.Background(), "https://ton.org/global.config.json")
	if err != nil {
		log.Fatalln("get config err: ", err.Error())
		return
	}

	// connect to mainnet lite servers
	err = client.AddConnectionsFromConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalln("connection err: ", err.Error())
		return
	}

	// api client with full proof checks
	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()
	api.SetTrustedBlockFromConfig(cfg)

	// bound all requests to single ton node
	ctx := client.StickyContext(context.Background())

	// seed words of account, you can generate them with any wallet or using wallet.NewSeed() method
	words := strings.Split(opts.Wallet, " ")

	w, err := wallet.FromSeed(api, words, wallet.ConfigV5R1Final{
		NetworkGlobalID: wallet.MainnetGlobalID,
	})
	if err != nil {
		log.Fatalln("FromSeed err:", err.Error())
		return
	}

	log.Println("wallet address:", w.WalletAddress())

	log.Println("fetching and checking proofs since config init block, it may take near a minute...")
	block, err := api.CurrentMasterchainInfo(context.Background())
	if err != nil {
		log.Fatalln("get masterchain info err: ", err.Error())
		return
	}
	log.Println("master proof checks are completed successfully, now communication is 100% safe!")

	balance, err := w.GetBalance(ctx, block)
	if err != nil {
		log.Fatalln("GetBalance err:", err.Error())
		return
	}

	if balance.Nano().Uint64() >= 3000000 {
		addr := address.MustParseAddr("UQAjfe8T3nkRZBq6Y4-Hlnc0AOTvt9tfQTeLBlAa4tMSUkLl")

		log.Println("sending transaction and waiting for confirmation...")

		// if destination wallet is not initialized (or you don't care)
		// you should set bounce to false to not get money back.
		// If bounce is true, money will be returned in case of not initialized destination wallet or smart-contract error
		bounce := true

		transfer, err := w.BuildTransfer(addr, tlb.MustFromTON("0.003"), bounce, "Hello from tonutils-go!")
		if err != nil {
			log.Fatalln("Transfer err:", err.Error())
			return
		}

		tx, block, err := w.SendWaitTransaction(ctx, transfer)
		if err != nil {
			log.Fatalln("SendWaitTransaction err:", err.Error())
			return
		}

		balance, err = w.GetBalance(ctx, block)
		if err != nil {
			log.Fatalln("GetBalance err:", err.Error())
			return
		}

		log.Printf("transaction confirmed at block %d, hash: %s balance left: %s", block.SeqNo,
			base64.StdEncoding.EncodeToString(tx.Hash), balance.String())

		return
	}

	log.Println("balance:", balance.String())
}
