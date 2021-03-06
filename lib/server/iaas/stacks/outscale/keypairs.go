/*
 * Copyright 2018-2020, CS Systemes d'Information, http://csgroup.eu
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package outscale

import (
	"encoding/base64"
	"fmt"

	"github.com/CS-SI/SafeScale/lib/utils/debug"

	"github.com/antihax/optional"

	"github.com/outscale-dev/osc-sdk-go/osc"

	"github.com/CS-SI/SafeScale/lib/server/iaas/abstract"
	"github.com/CS-SI/SafeScale/lib/utils/fail"
)

// CreateKeyPair creates and import a key pair
func (s *Stack) CreateKeyPair(name string) (*abstract.KeyPair, fail.Error) {
	tracer := debug.NewTracer(nil, fmt.Sprintf("(%s)", name), true).WithStopwatch().GoingIn()
	defer tracer.OnExitTrace()()

	keypair, err := abstract.NewKeyPair(name)
	if err != nil {
		return nil, err
	}
	return keypair, s.ImportKeyPair(keypair)
}

// ImportKeyPair is used to import an existing KeyPair in Outscale
func (s *Stack) ImportKeyPair(keypair *abstract.KeyPair) error {
	if keypair == nil {
		return fail.InvalidParameterError("keyair", "cannot be nil")
	}

	createKeypairRequest := osc.CreateKeypairRequest{
		KeypairName: keypair.Name,
		PublicKey:   base64.StdEncoding.EncodeToString([]byte(keypair.PublicKey)),
	}
	_, _, err := s.client.KeypairApi.CreateKeypair(
		s.auth, &osc.CreateKeypairOpts{
			CreateKeypairRequest: optional.NewInterface(createKeypairRequest),
		},
	)
	return normalizeError(err)
}

// GetKeyPair returns the key pair identified by id
func (s *Stack) GetKeyPair(id string) (*abstract.KeyPair, fail.Error) {
	readKeypairsRequest := osc.ReadKeypairsRequest{
		Filters: osc.FiltersKeypair{
			KeypairNames: []string{id},
		},
	}
	resp, _, err := s.client.KeypairApi.ReadKeypairs(
		s.auth, &osc.ReadKeypairsOpts{
			ReadKeypairsRequest: optional.NewInterface(readKeypairsRequest),
		},
	)
	if err != nil {
		return nil, normalizeError(err)
	}
	if len(resp.Keypairs) > 1 {
		return nil, fail.InconsistentError("Inconsistent provider response")
	}
	if len(resp.Keypairs) == 0 {
		return nil, fail.NotFoundError(fmt.Sprintf("Keypair %s not found", id))
	}
	kp := resp.Keypairs[0]
	return &abstract.KeyPair{
		ID:   kp.KeypairName,
		Name: kp.KeypairName,
	}, nil
}

// ListKeyPairs lists available key pairs
func (s *Stack) ListKeyPairs() ([]abstract.KeyPair, fail.Error) {
	resp, _, err := s.client.KeypairApi.ReadKeypairs(s.auth, nil)
	if err != nil {
		return nil, normalizeError(err)
	}
	var kps []abstract.KeyPair
	for _, kp := range resp.Keypairs {
		kps = append(
			kps, abstract.KeyPair{
				ID:   kp.KeypairName,
				Name: kp.KeypairName,
			},
		)
	}
	return kps, nil

}

// DeleteKeyPair deletes the key pair identified by id
func (s *Stack) DeleteKeyPair(name string) error {
	deleteKeypairRequest := osc.DeleteKeypairRequest{
		KeypairName: name,
	}
	_, _, err := s.client.KeypairApi.DeleteKeypair(
		s.auth, &osc.DeleteKeypairOpts{
			DeleteKeypairRequest: optional.NewInterface(deleteKeypairRequest),
		},
	)
	return normalizeError(err)
}
