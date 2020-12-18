/*
 * Copyright (c) 2018-2020 vChain, Inc. All Rights Reserved.
 * This software is released under GPL3.
 * The full license information can be found under:
 * https://www.gnu.org/licenses/gpl-3.0.en.html
 *
 */

package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	immuschema "github.com/codenotary/immudb/pkg/api/schema"
	"github.com/vchain-us/ledger-compliance-go/schema"
	"github.com/vchain-us/vcn/pkg/meta"
	"google.golang.org/grpc/metadata"
	"time"
)

func (a Artifact) toLcArtifact() *LcArtifact {
	aR := &LcArtifact{
		// root fields
		Kind:        a.Kind,
		Name:        a.Name,
		Hash:        a.Hash,
		Size:        a.Size,
		ContentType: a.ContentType,

		// custom metadata
		Metadata: a.Metadata,
	}

	return aR
}
func ItemToLcArtifact(item *schema.ItemExt) (*LcArtifact, error) {
	var lca LcArtifact
	err := json.Unmarshal(item.Item.Value, &lca)
	if err != nil {
		return nil, err
	}
	lca.Timestamp = time.Unix(int64(item.Timestamp.GetSeconds()), int64(item.Timestamp.GetNanos())).UTC()

	return &lca, nil
}

func ZItemToLcArtifact(ie *schema.ZItemExt) (*LcArtifact, error) {
	var lca LcArtifact
	err := json.Unmarshal(ie.Item.Item.Value, &lca)
	if err != nil {
		return nil, err
	}
	lca.Timestamp = time.Unix(int64(ie.Timestamp.GetSeconds()), int64(ie.Timestamp.GetNanos())).UTC()

	return &lca, nil
}

func ZStructuredItemToLcArtifact(i *immuschema.ZItem) (*LcArtifact, error) {
	var lca LcArtifact
	err := json.Unmarshal(i.Item.Value, &lca)
	if err != nil {
		return nil, err
	}
	timestamp := time.Unix(0, int64(i.Score))
	lca.Timestamp = timestamp.UTC()

	return &lca, nil
}

func ItemExtToLcArtifact(item *schema.ItemExt) (*LcArtifact, error) {
	var lca LcArtifact
	err := json.Unmarshal(item.Item.Value, &lca)
	if err != nil {
		return nil, err
	}
	lca.Timestamp = time.Unix(int64(item.Timestamp.GetSeconds()), int64(item.Timestamp.GetNanos())).UTC()
	return &lca, nil
}

type LcArtifact struct {
	// root fields
	Kind        string    `json:"kind" yaml:"kind" vcn:"Kind"`
	Name        string    `json:"name" yaml:"name" vcn:"Name"`
	Hash        string    `json:"hash" yaml:"hash" vcn:"Hash"`
	Size        uint64    `json:"size" yaml:"size" vcn:"Size"`
	Timestamp   time.Time `json:"timestamp" yaml:"timestamp" vcn:"Timestamp"`
	ContentType string    `json:"contentType" yaml:"contentType" vcn:"ContentType"`

	// custom metadata
	Metadata Metadata `json:"metadata" yaml:"metadata" vcn:"Metadata"`

	Signer string      `json:"signer" yaml:"signer" vcn:"Signer"`
	Status meta.Status `json:"status" yaml:"status" vcn:"Status"`
}

func (u LcUser) createArtifact(
	artifact Artifact, status meta.Status) error {

	aR := artifact.toLcArtifact()
	aR.Status = status

	hasher := sha256.New()
	hasher.Write([]byte(u.LcApiKey()))
	signerId := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	aR.Signer = signerId

	arJson, err := json.Marshal(aR)

	md := metadata.Pairs(meta.VcnLCPluginTypeHeaderName, meta.VcnLCPluginTypeHeaderValue)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	key := AppendPrefix(meta.VcnLCPrefix, []byte(aR.Signer))
	key = AppendSignerId(artifact.Hash, key)

	_, err = u.Client.Set(ctx, key, arJson)
	if err != nil {
		return err
	}
	return nil
}

// LoadArtifact fetches and returns an *lcArtifact for the given hash and current u, if any.
func (u *LcUser) LoadArtifact(hash, signerID string) (lc *LcArtifact, verified bool, err error) {

	md := metadata.Pairs(meta.VcnLCPluginTypeHeaderName, meta.VcnLCPluginTypeHeaderValue)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	if signerID == "" {
		hasher := sha256.New()
		hasher.Write([]byte(u.LcApiKey()))
		signerID = base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	}

	key := AppendPrefix(meta.VcnLCPrefix, []byte(signerID))
	key = AppendSignerId(hash, key)

	jsonAr, err := u.Client.VerifiedGetExt(ctx, key)
	if err != nil {
		return nil, false, err
	}

	lcArtifact, err := ItemExtToLcArtifact(jsonAr)
	if err != nil {
		return nil, false, err
	}

	return lcArtifact, true, nil
}

func AppendPrefix(prefix string, key []byte) []byte {
	var prefixed = make([]byte, len(prefix)+1+len(key))
	copy(prefixed[0:], prefix+".")
	copy(prefixed[len(prefix)+1:], key)
	return prefixed
}

func AppendSignerId(signerId string, k []byte) []byte {
	var prefixed = make([]byte, len(k)+len(signerId)+1)
	copy(prefixed[0:], k)
	copy(prefixed[len(k):], "."+signerId)
	return prefixed
}
