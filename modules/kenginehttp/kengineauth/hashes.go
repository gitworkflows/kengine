// Copyright 2015 Matthew Holt and The Kengine Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kengineauth

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(BcryptHash{})
}

// BcryptHash implements the bcrypt hash.
type BcryptHash struct{}

// KengineModule returns the Kengine module information.
func (BcryptHash) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.authentication.hashes.bcrypt",
		New: func() kengine.Module { return new(BcryptHash) },
	}
}

// Compare compares passwords.
func (BcryptHash) Compare(hashed, plaintext []byte) (bool, error) {
	err := bcrypt.CompareHashAndPassword(hashed, plaintext)
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Hash hashes plaintext using a random salt.
func (BcryptHash) Hash(plaintext []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(plaintext, 14)
}

// FakeHash returns a fake hash.
func (BcryptHash) FakeHash() []byte {
	// hashed with the following command:
	// kengine hash-password --plaintext "antitiming" --algorithm "bcrypt"
	return []byte("$2a$14$X3ulqf/iGxnf1k6oMZ.RZeJUoqI9PX2PM4rS5lkIKJXduLGXGPrt6")
}

// Interface guards
var (
	_ Comparer = (*BcryptHash)(nil)
	_ Hasher   = (*BcryptHash)(nil)
)
