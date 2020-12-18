/*
 * Copyright (c) 2018-2020 vChain, Inc. All Rights Reserved.
 * This software is released under GPL3.
 * The full license information can be found under:
 * https://www.gnu.org/licenses/gpl-3.0.en.html
 *
 */

package inspect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	immuschema "github.com/codenotary/immudb/pkg/api/schema"
	"github.com/vchain-us/ledger-compliance-go/schema"
	"github.com/vchain-us/vcn/pkg/api"
	"github.com/vchain-us/vcn/pkg/cmd/internal/cli"
	"github.com/vchain-us/vcn/pkg/cmd/internal/types"
	"github.com/vchain-us/vcn/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"
)

func lcInspect(hash string, signerID string, u *api.LcUser, first, last uint64, start, end string, output string) (err error) {
	hasher := sha256.New()
	hasher.Write([]byte(u.LcApiKey()))
	contextSignerID := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	if signerID == "" {
		if output == "" {
			fmt.Println("no signer ID provided. Full history of the item is returned")
		}
	} else {
		contextSignerID = signerID
	}

	results, err := GetLcResults(hash, signerID, u, first, last, start, end)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			if s.Code() == codes.ResourceExhausted {
				return fmt.Errorf("too many notarizations are returned. Try to use --first or --last filter or datetime range filter")
			}
		}
		return err
	}
	l := len(results)
	if output == "" {
		fmt.Printf(
			`current signer ID "%s"
%d notarizations found for "%s"

`,
			contextSignerID, l, hash)
	}

	return cli.PrintLcSlice(output, results)
}

func GetLcResults(hash, signerID string, u *api.LcUser, first, last uint64, start, end string) (results []*types.LcResult, err error) {
	md := metadata.Pairs(meta.VcnLCPluginTypeHeaderName, meta.VcnLCPluginTypeHeaderValue)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	var key []byte
	if signerID == "" {
		key = []byte(hash)
	} else {
		key = api.AppendPrefix(meta.VcnLCPrefix, []byte(signerID))
		key = api.AppendSignerId(hash, key)
	}

	if start != "" || end != "" {
		if signerID == "" {
			key = append([]byte(meta.IndexDateRangePrefix), key...)
		}
		results, err = getTimeRangedResults(ctx, u, key, first, last, start, end)
		if err != nil {
			return nil, err
		}
	} else {
		if signerID == "" {
			results, err = getSignerResults(ctx, key, u, first, last)
			if err != nil {
				return nil, err
			}
		} else {
			results, err = getHistoryResults(ctx, key, u, first, last)
			if err != nil {
				return nil, err
			}
		}
	}

	return results, nil
}

func getSignerResults(ctx context.Context, key []byte, u *api.LcUser, first, last uint64) ([]*types.LcResult, error) {
	var err error
	var zitems *schema.ZItemExtList

	reverse := false
	var limit uint64 = 0

	if first > 0 {
		limit = first
	}
	if last > 0 {
		limit = last
		reverse = true
	}

	zitems, err = u.Client.ZScanExt(ctx, &immuschema.ZScanRequest{
		Desc:  reverse,
		Limit: limit,
		Set:   key,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*types.LcResult, len(zitems.Items))
	var i = 0
	for _, v := range zitems.Items {
		lca, err := api.ZItemToLcArtifact(v)
		if err != nil {
			results[i].AddError(err)
		}
		results[i] = types.NewLcResult(lca, true)

		i++
	}
	return results, nil
}

func getHistoryResults(ctx context.Context, key []byte, u *api.LcUser, first, last uint64) ([]*types.LcResult, error) {
	var err error
	var items *schema.ItemExtList

	reverse := true
	var limit uint64 = 0

	if first > 0 {
		limit = first
	}
	if last > 0 {
		limit = last
		reverse = false
	}

	items, err = u.Client.HistoryExt(ctx, &immuschema.HistoryRequest{
		Desc:  reverse,
		Limit: limit,
		Key:   key,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*types.LcResult, len(items.Items))
	var i = 0
	for _, v := range items.Items {
		lca, err := api.ItemToLcArtifact(v)
		if err != nil {
			return nil, err
		}
		results[i] = types.NewLcResult(lca, true)
		if err != nil {
			results[i].AddError(err)
		}
		i++
	}
	return results, nil
}

func getTimeRangedResults(ctx context.Context, u *api.LcUser, set []byte, first, last uint64, start, end string) ([]*types.LcResult, error) {
	var err error
	var zitems *immuschema.ZItemList

	var startScore *immuschema.Score = nil
	var endScore *immuschema.Score = nil

	if start != "" {
		timeStart, err := time.Parse(meta.DateShortForm, start)
		if err != nil {
			return nil, err
		}
		startScore = &immuschema.Score{
			Score: float64(timeStart.UnixNano()), // there is no precision loss. 52 bit are enough to represent seconds.
		}
	}

	if end != "" {
		timeEnd, err := time.Parse(meta.DateShortForm, end)
		if err != nil {
			return nil, err
		}
		endScore = &immuschema.Score{
			Score: float64(timeEnd.UnixNano()), // there is no precision loss. 52 bit are enough to represent seconds.
		}
	}

	reverse := false
	var limit uint64 = 0

	if first > 0 {
		limit = first
	}
	if last > 0 {
		limit = last
		reverse = true
	}

	zitems, err = u.Client.ZScan(ctx, &immuschema.ZScanRequest{
		Set:      set,
		MinScore: startScore,
		MaxScore: endScore,
		Limit:    limit,
		Desc:     reverse,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*types.LcResult, len(zitems.Items))
	var i = 0
	for _, v := range zitems.Items {
		lca, err := api.ZStructuredItemToLcArtifact(v)
		if err != nil {
			return nil, err
		}
		results[i] = types.NewLcResult(lca, true)
		if err != nil {
			results[i].AddError(err)
		}
		i++
	}
	return results, nil
}
