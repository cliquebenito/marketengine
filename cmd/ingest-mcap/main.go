package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"marketengine/internal/repo/webapi/coinglass"
	"marketengine/internal/storage"

	"github.com/jackc/pgx/v5"
)

var symbolToCoinID = map[string]string{
	"BTC": "bitcoin",
	"ETH": "ethereum",
}

func main() {
	var (
		dbURL   = flag.String("db", "postgres://regime:regime@localhost:5432/regime?sslmode=disable", "DB URL")
		symbols = flag.String("symbols", "BTC,ETH", "comma-separated CoinGlass symbols")
		keyF    = flag.String("api-key", "", "CoinGlass API key (falls back to COINGLASS_API_KEY env)")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	apiKey := os.Getenv("COINGLASS_API_KEY")
	if *keyF != "" {
		apiKey = *keyF
	}
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "COINGLASS_API_KEY env or -api-key flag required\n")
		os.Exit(2)
	}

	ctx := context.Background()
	pool, err := storage.Open(ctx, *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	cg := coinglass.New(apiKey, 60*time.Second)

	for _, sym := range strings.Split(*symbols, ",") {
		sym = strings.TrimSpace(strings.ToUpper(sym))
		if sym == "" {
			continue
		}
		coinID, ok := symbolToCoinID[sym]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown symbol %s (no mapping to coin_id)\n", sym)
			os.Exit(2)
		}
		points, err := cg.FetchMarketDataHistory(ctx, sym)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch %s: %v\n", sym, err)
			os.Exit(1)
		}
		err = pool.InTx(ctx, func(tx pgx.Tx) error {
			for _, p := range points {
				row := storage.RawMarketCap{
					ValueDate:     p.Timestamp,
					CoinID:        coinID,
					MarketCapUSD:  p.MarketCapUSD,
					PriceUSD:      p.PriceUSD,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				}
				if err := storage.InsertRawMarketCap(ctx, tx, row); err != nil {
					return fmt.Errorf("insert %s: %w", coinID, err)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", sym, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "%s → coin_id=%s: %d rows written (%s → %s)\n",
			sym, coinID, len(points),
			points[0].Timestamp.Format("2006-01-02"),
			points[len(points)-1].Timestamp.Format("2006-01-02"))
	}
}
