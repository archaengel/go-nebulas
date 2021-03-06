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

package core

import (
	"fmt"
	"time"

	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/nebulasio/go-nebulas/core/pb"
	"github.com/nebulasio/go-nebulas/crypto"
	"github.com/nebulasio/go-nebulas/crypto/hash"
	"github.com/nebulasio/go-nebulas/crypto/keystore"
	"github.com/nebulasio/go-nebulas/util"
	"github.com/nebulasio/go-nebulas/util/byteutils"
	"github.com/nebulasio/go-nebulas/util/logging"
	"github.com/sirupsen/logrus"
)

const (
	// TxHashByteLength invalid tx hash length(len of []byte)
	TxHashByteLength = 32
)

var (
	// TransactionMaxGasPrice max gasPrice:50 * 10 ** 9
	TransactionMaxGasPrice, _ = util.NewUint128FromString("50000000000")

	// TransactionMaxGas max gas:50 * 10 ** 9
	TransactionMaxGas, _ = util.NewUint128FromString("50000000000")

	// TransactionGasPrice default gasPrice : 10**6
	TransactionGasPrice, _ = util.NewUint128FromInt(1000000)

	// MinGasCountPerTransaction default gas for normal transaction
	MinGasCountPerTransaction, _ = util.NewUint128FromInt(20000)

	// GasCountPerByte per byte of data attached to a transaction gas cost
	GasCountPerByte, _ = util.NewUint128FromInt(1)

	// MaxDataPayLoadLength Max data length in transaction
	MaxDataPayLoadLength = 1024 * 1024
)

// TransactionEvent transaction event
type TransactionEvent struct {
	Hash    string `json:"hash"`
	Status  int8   `json:"status"`
	GasUsed string `json:"gas_used"`
	Error   string `json:"error"`
}

// Transaction type is used to handle all transaction data.
type Transaction struct {
	hash      byteutils.Hash
	from      *Address
	to        *Address
	value     *util.Uint128
	nonce     uint64
	timestamp int64
	data      *corepb.Data
	chainID   uint32
	gasPrice  *util.Uint128
	gasLimit  *util.Uint128

	// Signature
	alg  keystore.Algorithm
	sign byteutils.Hash // Signature values
}

// From return from address
func (tx *Transaction) From() *Address {
	return tx.from
}

// Timestamp return timestamp
func (tx *Transaction) Timestamp() int64 {
	return tx.timestamp
}

// To return to address
func (tx *Transaction) To() *Address {
	return tx.to
}

// ChainID return chainID
func (tx *Transaction) ChainID() uint32 {
	return tx.chainID
}

// Value return tx value
func (tx *Transaction) Value() *util.Uint128 {
	return tx.value
}

// Nonce return tx nonce
func (tx *Transaction) Nonce() uint64 {
	return tx.nonce
}

// Type return tx type
func (tx *Transaction) Type() string {
	return tx.data.Type
}

// Data return tx data
func (tx *Transaction) Data() []byte {
	return tx.data.Payload
}

// ToProto converts domain Tx to proto Tx
func (tx *Transaction) ToProto() (proto.Message, error) {
	value, err := tx.value.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	gasPrice, err := tx.gasPrice.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	gasLimit, err := tx.gasLimit.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	return &corepb.Transaction{
		Hash:      tx.hash,
		From:      tx.from.address,
		To:        tx.to.address,
		Value:     value,
		Nonce:     tx.nonce,
		Timestamp: tx.timestamp,
		Data:      tx.data,
		ChainId:   tx.chainID,
		GasPrice:  gasPrice,
		GasLimit:  gasLimit,
		Alg:       uint32(tx.alg),
		Sign:      tx.sign,
	}, nil
}

// FromProto converts proto Tx into domain Tx
func (tx *Transaction) FromProto(msg proto.Message) error {
	if msg, ok := msg.(*corepb.Transaction); ok {
		tx.hash = msg.Hash

		from, err := AddressParseFromBytes(msg.From)
		if err != nil {
			return err
		}
		tx.from = from

		to, err := AddressParseFromBytes(msg.To)
		if err != nil {
			return err
		}
		tx.to = to

		value, err := util.NewUint128FromFixedSizeByteSlice(msg.Value)
		if err != nil {
			return err
		}
		tx.value = value
		tx.nonce = msg.Nonce
		tx.timestamp = msg.Timestamp

		data := msg.Data
		if data == nil {
			return ErrInvalidTransactionData
		}
		if len(data.Payload) > MaxDataPayLoadLength {
			return ErrTxDataPayLoadOutOfMaxLength
		}

		tx.data = msg.Data
		tx.chainID = msg.ChainId
		gasPrice, err := util.NewUint128FromFixedSizeByteSlice(msg.GasPrice)
		if err != nil {
			return err
		}
		tx.gasPrice = gasPrice
		gasLimit, err := util.NewUint128FromFixedSizeByteSlice(msg.GasLimit)
		if err != nil {
			return err
		}
		tx.gasLimit = gasLimit
		tx.alg = keystore.Algorithm(msg.Alg)
		tx.sign = msg.Sign
		return nil
	}
	return ErrCannotConvertTransaction
}

func (tx *Transaction) String() string {
	return fmt.Sprintf(`{"chainID":%d, "hash":"%s", "from":"%s", "to":"%s", "nonce":%d, "value":"%s", "timestamp":%d, "gasprice": "%s", "gaslimit":"%s", "type":"%s"}`,
		tx.chainID,
		tx.hash.String(),
		tx.from.String(),
		tx.to.String(),
		tx.nonce,
		tx.value.String(),
		tx.timestamp,
		tx.gasPrice.String(),
		tx.gasLimit.String(),
		tx.Type(),
	)
}

// Transactions is an alias of Transaction array.
type Transactions []*Transaction

// NewTransaction create #Transaction instance.
func NewTransaction(chainID uint32, from, to *Address, value *util.Uint128, nonce uint64, payloadType string, payload []byte, gasPrice *util.Uint128, gasLimit *util.Uint128) (*Transaction, error) {
	//if gasPrice is not specified, use the default gasPrice
	if gasPrice == nil || gasPrice.Cmp(util.NewUint128()) <= 0 {
		gasPrice = TransactionGasPrice
	}
	if gasLimit == nil || gasLimit.Cmp(util.NewUint128()) <= 0 {
		gasLimit = MinGasCountPerTransaction
	}

	if nil == from || nil == to || nil == value {
		logging.VLog().WithFields(logrus.Fields{
			"from":  from,
			"to":    to,
			"value": value,
		}).Error("invalid parameters")
		return nil, ErrInvalidArgument
	}

	if len(payload) > MaxDataPayLoadLength {
		return nil, ErrTxDataPayLoadOutOfMaxLength
	}

	tx := &Transaction{
		from:      from,
		to:        to,
		value:     value,
		nonce:     nonce,
		timestamp: time.Now().Unix(),
		chainID:   chainID,
		data:      &corepb.Data{Type: payloadType, Payload: payload},
		gasPrice:  gasPrice,
		gasLimit:  gasLimit,
	}
	return tx, nil
}

// Hash return the hash of transaction.
func (tx *Transaction) Hash() byteutils.Hash {
	return tx.hash
}

// GasPrice returns gasPrice
func (tx *Transaction) GasPrice() *util.Uint128 {
	return tx.gasPrice
}

// GasLimit returns gasLimit
func (tx *Transaction) GasLimit() *util.Uint128 {
	return tx.gasLimit
}

// PayloadGasLimit returns payload gasLimit
func (tx *Transaction) PayloadGasLimit(payload TxPayload) (*util.Uint128, error) {
	if payload == nil {
		return nil, ErrNilArgument
	}

	// payloadGasLimit = tx.gasLimit - tx.GasCountOfTxBase
	gasCountOfTxBase, err := tx.GasCountOfTxBase()
	if err != nil {
		return nil, err
	}
	payloadGasLimit, err := tx.gasLimit.Sub(gasCountOfTxBase)
	if err != nil {
		return nil, ErrOutOfGasLimit
	}
	payloadGasLimit, err = payloadGasLimit.Sub(payload.BaseGasCount())
	if err != nil {
		return nil, ErrOutOfGasLimit
	}
	return payloadGasLimit, nil
}

// MinBalanceRequired returns gasprice * gaslimit + tx.value.
func (tx *Transaction) MinBalanceRequired() (*util.Uint128, error) {
	total, err := tx.GasPrice().Mul(tx.GasLimit())
	if err != nil {
		return nil, err
	}
	total, err = total.Add(tx.value)
	if err != nil {
		return nil, err
	}
	return total, nil
}

// GasCountOfTxBase calculate the actual amount for a tx with data
func (tx *Transaction) GasCountOfTxBase() (*util.Uint128, error) {
	txGas := MinGasCountPerTransaction.DeepCopy()
	if tx.DataLen() > 0 {
		dataLen, err := util.NewUint128FromInt(int64(tx.DataLen()))
		if err != nil {
			return nil, err
		}
		dataGas, err := dataLen.Mul(GasCountPerByte)
		if err != nil {
			return nil, err
		}
		txGas, err = txGas.Add(dataGas)
		if err != nil {
			return nil, err
		}
	}
	return txGas, nil
}

// DataLen return the length of payload
func (tx *Transaction) DataLen() int {
	return len(tx.data.Payload)
}

// LoadPayload returns tx's payload
func (tx *Transaction) LoadPayload() (TxPayload, error) {
	// execute payload
	var (
		payload TxPayload
		err     error
	)
	switch tx.data.Type {
	case TxPayloadBinaryType:
		payload, err = LoadBinaryPayload(tx.data.Payload)
	case TxPayloadDeployType:
		payload, err = LoadDeployPayload(tx.data.Payload)
	case TxPayloadCallType:
		payload, err = LoadCallPayload(tx.data.Payload)
	default:
		err = ErrInvalidTxPayloadType
	}
	return payload, err
}

// LocalExecution returns tx local execution
func (tx *Transaction) LocalExecution(block *Block) (*util.Uint128, string, error) {
	if block == nil {
		return nil, "", ErrNilArgument
	}

	txBlock, err := block.Clone()
	if err != nil {
		return nil, "", err
	}

	txBlock.begin()
	defer txBlock.rollback()

	payload, err := tx.LoadPayload()
	if err != nil {
		return nil, "", err
	}

	gasUsed, err := tx.GasCountOfTxBase()
	if err != nil {
		return nil, "", err
	}
	gasUsed, err = gasUsed.Add(payload.BaseGasCount())
	if err != nil {
		return nil, "", err
	}

	gasExecution, result, exeErr := payload.Execute(txBlock, tx)

	gasUsed, err = gasUsed.Add(gasExecution)
	if err != nil {
		return nil, result, err
	}
	return gasUsed, result, exeErr
}

// VerifyExecution transaction and return result.
func (tx *Transaction) VerifyExecution(block *Block) (*util.Uint128, error) {
	if block == nil {
		return nil, ErrNilArgument
	}

	// step1. check gasLimit >= GasCountOfTxBase()
	gasUsed, err := tx.GasCountOfTxBase()
	if err != nil {
		return nil, err
	}
	if tx.gasLimit.Cmp(gasUsed) < 0 {
		logging.VLog().WithFields(logrus.Fields{
			"error":       ErrOutOfGasLimit,
			"transaction": tx,
			"limit":       tx.gasLimit,
			"used":        gasUsed,
		}).Debug("Failed to check gasLimit.")
		return nil, ErrOutOfGasLimit
	}

	// step2. check balance >= gasLimit*gasPric + tx.value
	minBalanceRequired, err := tx.MinBalanceRequired()
	if err != nil {
		return nil, err
	}
	fromAcc, err := block.accState.GetOrCreateUserAccount(tx.from.address)
	if err != nil {
		return nil, err
	}
	if fromAcc.Balance().Cmp(minBalanceRequired) < 0 {
		logging.VLog().WithFields(logrus.Fields{
			"from":               fromAcc,
			"minBalanceRequired": minBalanceRequired,
			"error":              ErrInsufficientBalance,
			"transaction":        tx,
			"limit":              tx.gasLimit.String(),
			"used":               gasUsed.String(),
		}).Debug("Failed to check from balance.")
		return nil, ErrInsufficientBalance
	}

	// step3. check payload vaild
	payload, payloadErr := tx.LoadPayload()
	if payloadErr != nil {
		logging.VLog().WithFields(logrus.Fields{
			"payloadErr":  payloadErr,
			"block":       block,
			"transaction": tx,
		}).Debug("Failed to load payload.")

		gas, err := tx.gasPrice.Mul(gasUsed)
		if err != nil {
			return nil, err
		}
		if err := tx.transfer(block, tx.from, block.Coinbase(), gas); err != nil {
			return nil, err
		}
		if err := tx.recordResultEvent(block, gasUsed, payloadErr); err != nil {
			return nil, err
		}

		metricsTxExeFailed.Mark(1)
		return gasUsed, nil
	}

	// step4. check gasLimit > gas + payload.baseGasCount
	gasUsed, err = gasUsed.Add(payload.BaseGasCount())
	if err != nil {
		return nil, err
	}
	if tx.gasLimit.Cmp(gasUsed) < 0 {
		logging.VLog().WithFields(logrus.Fields{
			"err":   ErrOutOfGasLimit,
			"block": block,
			"tx":    tx,
		}).Debug("Failed to check payload gas used.")

		gas, err := tx.gasPrice.Mul(tx.gasLimit)
		if err != nil {
			return nil, err
		}
		if err := tx.transfer(block, tx.from, block.Coinbase(), gas); err != nil {
			return nil, err
		}
		if err := tx.recordResultEvent(block, tx.gasLimit, ErrOutOfGasLimit); err != nil {
			return nil, err
		}

		metricsTxExeFailed.Mark(1)
		return tx.gasLimit, nil
	}

	// step5. transfer tx value
	// block begin
	txBlock, err := block.Clone()
	if err != nil {
		return util.NewUint128(), err
	}

	if err := tx.transfer(txBlock, tx.from, tx.to, tx.value); err != nil {
		return nil, err
	}

	// step6. execute payload
	// execute smart contract and sub the calcute gas.
	gasExecution, _, exeErr := payload.Execute(txBlock, tx)

	// step7. gas + gasExecution
	// gas = tx.GasCountOfTxBase() +  gasExecution
	gasUsed, gasErr := gasUsed.Add(gasExecution)
	if gasErr != nil {
		return nil, gasErr
	}

	if tx.gasLimit.Cmp(gasUsed) < 0 {
		gasUsed = tx.gasLimit
		exeErr = ErrOutOfGasLimit
	}

	// only execute success, merge the state to use
	if exeErr == nil {
		block.Merge(txBlock)
	}

	// step8. consume gas
	gas, err := tx.gasPrice.Mul(gasUsed)
	if err != nil {
		return nil, err
	}
	if err := tx.transfer(block, tx.from, block.Coinbase(), gas); err != nil {
		return nil, err
	}

	if exeErr != nil {
		logging.VLog().WithFields(logrus.Fields{
			"exeErr":       exeErr,
			"block":        block,
			"tx":           tx,
			"gasUsed":      gasUsed,
			"gasExecution": gasExecution,
		}).Debug("Failed to execute payload.")

		metricsTxExeFailed.Mark(1)
	} else {
		metricsTxExeSuccess.Mark(1)
	}

	if err := tx.recordResultEvent(block, gas, exeErr); err != nil {
		return nil, err
	}

	return gasUsed, nil
}

func (tx *Transaction) transfer(block *Block, from, to *Address, value *util.Uint128) error {
	fromAcc, err := block.accState.GetOrCreateUserAccount(from.address)
	if err != nil {
		return err
	}

	toAcc, err := block.accState.GetOrCreateUserAccount(to.address)
	if err != nil {
		return err
	}

	err = fromAcc.SubBalance(value)
	if err != nil {
		return err
	}
	err = toAcc.AddBalance(value)
	return err
}

func (tx *Transaction) recordResultEvent(block *Block, gasUsed *util.Uint128, err error) error {

	txEvent := &TransactionEvent{
		Hash:    tx.hash.String(),
		GasUsed: gasUsed.String(),
	}
	if err != nil {
		txEvent.Status = TxExecutionFailed
		txEvent.Error = err.Error()
	} else {
		txEvent.Status = TxExecutionSuccess
	}

	txData, err := json.Marshal(txEvent)
	if err != nil {
		return err
	}

	event := &Event{
		Topic: TopicTransactionExecutionResult,
		Data:  string(txData)}
	return block.recordEvent(tx.hash, event)
}

// Sign sign transaction,sign algorithm is
func (tx *Transaction) Sign(signature keystore.Signature) error {
	if signature == nil {
		return ErrNilArgument
	}
	hash, err := HashTransaction(tx)
	if err != nil {
		return err
	}
	sign, err := signature.Sign(hash)
	if err != nil {
		return err
	}
	tx.hash = hash
	tx.alg = signature.Algorithm()
	tx.sign = sign
	return nil
}

// VerifyIntegrity return transaction verify result, including Hash and Signature.
func (tx *Transaction) VerifyIntegrity(chainID uint32) error {
	// check ChainID.
	if tx.chainID != chainID {
		return ErrInvalidChainID
	}

	// check Hash.
	wantedHash, err := HashTransaction(tx)
	if err != nil {
		return err
	}
	if wantedHash.Equals(tx.hash) == false {
		return ErrInvalidTransactionHash
	}

	// check Signature.
	return tx.verifySign()

}

func (tx *Transaction) verifySign() error {
	signature, err := crypto.NewSignature(tx.alg)
	if err != nil {
		return err
	}
	pub, err := signature.RecoverPublic(tx.hash, tx.sign)
	if err != nil {
		return err
	}
	pubdata, err := pub.Encoded()
	if err != nil {
		return err
	}
	addr, err := NewAddressFromPublicKey(pubdata)
	if err != nil {
		return err
	}
	if !tx.from.Equals(addr) {
		logging.VLog().WithFields(logrus.Fields{
			"recover address": addr.String(),
			"tx":              tx,
		}).Debug("Failed to verify tx's sign.")
		return ErrInvalidTransactionSigner
	}
	return nil
}

// GenerateContractAddress according to tx.from and tx.nonce.
func (tx *Transaction) GenerateContractAddress() (*Address, error) {
	return NewContractAddressFromHash(hash.Sha3256(tx.from.Bytes(), byteutils.FromUint64(tx.nonce)))
}

// HashTransaction hash the transaction.
func HashTransaction(tx *Transaction) (byteutils.Hash, error) {
	value, err := tx.value.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(tx.data)
	if err != nil {
		return nil, err
	}
	gasPrice, err := tx.gasPrice.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	gasLimit, err := tx.gasLimit.ToFixedSizeByteSlice()
	if err != nil {
		return nil, err
	}
	return hash.Sha3256(
		tx.from.address,
		tx.to.address,
		value,
		byteutils.FromUint64(tx.nonce),
		byteutils.FromInt64(tx.timestamp),
		data,
		byteutils.FromUint32(tx.chainID),
		gasPrice,
		gasLimit,
	), nil
}
