package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/config"
	"phmon/server/internal/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 || os.Args[1] != "agent" || os.Args[2] != "create" {
		return errors.New("usage: phmonctl agent create [--json]")
	}
	jsonOutput := false
	for _, arg := range os.Args[3:] {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		return fmt.Errorf("unknown argument %q", arg)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("cannot initialize database pool")
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		return errors.New("database migrations failed")
	}

	credential, err := agents.NewCredential()
	if err != nil {
		return errors.New("cannot generate agent credential")
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		return errors.New("cannot store agent credential")
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"agent_id":    credential.AgentID,
			"agent_token": credential.Token,
		})
	}
	fmt.Printf("agent_id=%s\nagent_token=%s\n", credential.AgentID, credential.Token)
	return nil
}
