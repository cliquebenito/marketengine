package backtest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"marketengine/internal/domain"
)

func SnapshotHash(perDay map[SnapshotKey]map[domain.DomainCode]float64) string {
	keys := make([]SnapshotKey, 0, len(perDay))
	for k := range perDay {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Asset != keys[j].Asset {
			return keys[i].Asset < keys[j].Asset
		}
		return keys[i].ValueDate.Before(keys[j].ValueDate)
	})
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%s|%s", k.Asset, k.ValueDate.Format("2006-01-02"))
		scores := perDay[k]
		doms := make([]string, 0, len(scores))
		for d := range scores {
			doms = append(doms, string(d))
		}
		sort.Strings(doms)
		for _, d := range doms {
			fmt.Fprintf(h, "|%s=%.12f", d, scores[domain.DomainCode(d)])
		}
		fmt.Fprintln(h)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

type SnapshotKey struct {
	Asset     domain.Asset
	ValueDate time.Time
}
