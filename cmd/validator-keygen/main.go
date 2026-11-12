// Command validator-keygen creates the Ed25519 key pair used by the PoA
// validator gateway. Run it once per deployment and keep the private seed in
// /home/medvault/.env on the VPS only.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	short := flag.Bool("short", false, "print private and public key on one line")
	writeEnv := flag.String("write-env", "", "append both variables to an env file")
	flag.Parse()

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatalf("generate ed25519 key: %v", err)
	}

	privHex := hex.EncodeToString(priv.Seed())
	pubHex := hex.EncodeToString(priv.Public().(ed25519.PublicKey))

	if *writeEnv != "" {
		existing, err := os.ReadFile(*writeEnv)
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("read %s: %v", *writeEnv, err)
		}
		if strings.Contains(string(existing), "MEDVAULT_VALIDATOR_PRIVKEY_HEX=") {
			log.Fatalf("%s already defines MEDVAULT_VALIDATOR_PRIVKEY_HEX", *writeEnv)
		}

		f, err := os.OpenFile(*writeEnv, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			log.Fatalf("open %s: %v", *writeEnv, err)
		}
		if _, err := fmt.Fprintf(f,
			"MEDVAULT_VALIDATOR_PRIVKEY_HEX=%s\nMEDVAULT_VALIDATOR_PUBKEY_HEX=%s\n",
			privHex, pubHex); err != nil {
			f.Close()
			log.Fatalf("write %s: %v", *writeEnv, err)
		}
		if err := f.Close(); err != nil {
			log.Fatalf("close %s: %v", *writeEnv, err)
		}
		fmt.Printf("validator key pair written to %s\n", *writeEnv)
		return
	}

	if *short {
		fmt.Printf("%s %s\n", privHex, pubHex)
		return
	}

	fmt.Printf("MEDVAULT_VALIDATOR_PRIVKEY_HEX=%s\n", privHex)
	fmt.Printf("MEDVAULT_VALIDATOR_PUBKEY_HEX=%s\n", pubHex)
	fmt.Printf("MEDVAULT_VALIDATOR_PUBKEY_HEX is embedded in the app environment for verification\n")
}
