// Copyright (C) 2017 go-nebulas authors
//
// This file is part of the go-nebulas library.
//
// the go-nebulas library is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// the go-nebulas library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with the go-nebulas library.  If not, see <http://www.gnu.org/licenses/>.
//

package nvm

import (
	"github.com/nebulasio/go-nebulas/core"
	"github.com/nebulasio/go-nebulas/core/state"
)

// NebulasVM type of NebulasVM
type NebulasVM struct{}

// NewNebulasVM create new NebulasVM
func NewNebulasVM() core.NVM {
	return &NebulasVM{}
}

// CreateEngine start engine
func (nvm *NebulasVM) CreateEngine(block *core.Block, tx *core.Transaction, owner, contract state.Account, state state.TxWorldState) (core.SmartContractEngine, error) {
	return NewV8Engine(&Context{block, tx, owner, contract, state}), nil
}