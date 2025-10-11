package main

import (
	"fmt"
	"log"

	"github.com/aynakeya/bincfg"
)

type AppConfig struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
	Debug    bool   `json:"debug"`
}

func main() {

	var cfg AppConfig
	if err := bincfg.ReadJSON(&cfg); err != nil {
		if err == bincfg.ErrNotFound {
			cfg = AppConfig{Endpoint: "https://api.example.com", Debug: true}
			fmt.Println("no embedded config; using defaults")
		} else {
			log.Fatalf("read config: %v", err)
		}
	}
	fmt.Printf("running with cfg: %+v\n", cfg)
	//
	cfg.Debug = !cfg.Debug
	if err := bincfg.WriteJSON(cfg, true); err != nil {
		if err == bincfg.ErrPendingReplace {
			// Windows
			fmt.Println("config saved to .new; restart to finalize on Windows")
		} else {
			log.Fatalf("write config: %v", err)
		}
	} else {
		fmt.Println("config saved into the binary")
	}
}
