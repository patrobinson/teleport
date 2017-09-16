// +build dynamodb

/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package dynamo

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type mockKmsClient struct {
	kmsiface.KMSAPI
	Datakey []byte
}

func (k *mockKmsClient) GenerateDataKey(*kms.GenerateDataKeyInput) (*kms.GenerateDataKeyOutput, error) {
	return &kms.GenerateDataKeyOutput{
		KeyId:          aws.String("abcde-12345"),
		Plaintext:      k.Datakey,
		CiphertextBlob: []byte("ciphertext"),
	}, nil
}

func (k *mockKmsClient) Decrypt(*kms.DecryptInput) (*kms.DecryptOutput, error) {
	return &kms.DecryptOutput{
		Plaintext: k.Datakey,
	}, nil
}

func TestDynamoDB(t *testing.T) { TestingT(t) }

type DynamoDBSuite struct {
	bk    *DynamoDBBackend
	suite test.BackendSuite
	cfg   config
}

type config struct {
	backend.Config
	DynamoConfig
}

var _ = Suite(&DynamoDBSuite{})

func (s *DynamoDBSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()

	var err error
	cfg := make(backend.Params)
	cfg["type"] = "dynamodb"
	cfg["table_name"] = "teleport.dynamo.test"
	cfg["endpoint"] = "http://localhost:8000"
	cfg["access_key"] = "access_key"
	cfg["secret_key"] = "secret_key"

	backend, err := New(cfg)
	c.Assert(err, IsNil)
	s.bk = backend.(*DynamoDBBackend)
	s.bk.kmsSvc = &mockKmsClient{Datakey: []byte("example key 1234")}
	s.bk.generateDataKey("alias/teleport")
	s.suite.B = s.bk
}

func (s *DynamoDBSuite) TearDownSuite(c *C) {
	if s.bk != nil && s.bk.svc != nil {
		s.bk.deleteTable(s.cfg.Tablename, false)
	}
}

func (s *DynamoDBSuite) TestMigration(c *C) {
	cfg := make(backend.Params)
	cfg["type"] = "dynamodb"
	cfg["table_name"] = "teleport.dynamo.test"
	cfg["endpoint"] = "http://localhost:8000"
	cfg["access_key"] = "access_key"
	cfg["secret_key"] = "secret_key"
	// migration uses its own instance of the backend:
	backend, err := New(cfg)
	c.Assert(err, IsNil)
	bk := backend.(*DynamoDBBackend)
	s.bk.kmsSvc = &mockKmsClient{Datakey: []byte("example key 1234")}
	s.bk.generateDataKey("alias/teleport")

	var (
		legacytable      = "legacy.teleport.t"
		nonExistingTable = "nonexisting.teleport.t"
	)
	bk.deleteTable(legacytable, true)
	bk.deleteTable(legacytable+".bak", false)
	defer bk.deleteTable(legacytable, false)
	defer bk.deleteTable(legacytable+".bak", false)

	status, err := bk.getTableStatus(nonExistingTable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusMissing))

	err = bk.createTable(legacytable, oldPathAttr)
	c.Assert(err, IsNil)

	status, err = bk.getTableStatus(legacytable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusNeedsMigration))

	err = bk.migrate(legacytable)
	c.Assert(err, IsNil)

	status, err = bk.getTableStatus(legacytable)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, tableStatus(tableStatusOK))
}

func (s *DynamoDBSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *DynamoDBSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *DynamoDBSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *DynamoDBSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *DynamoDBSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTL(c)
}
func TestRecord(t *testing.T) {
	r := record{
		HashKey:   "teleport",
		FullPath:  "a/directory/path",
		Value:     []byte(`{"some":"json","data":true}`),
		Timestamp: int64(1505543201),
		TTL:       time.Duration(1) * time.Second,
	}

	r.encrypt([]byte("encrypteddatakey"), []byte("example key 1234"), "abcde-12345")
	if reflect.DeepEqual(r.Value, []byte(`{"some":"json","data":true}`)) {
		t.Error("Expected record to be encrypted but it was not")
	}
	firstEncryptedValue := r.Value
	r.decrypt([]byte("example key 1234"))
	r.encrypt([]byte("encrypteddatakey"), []byte("example key 1234"), "abcde-12345")
	if reflect.DeepEqual(r.Value, firstEncryptedValue) {
		t.Error("Expected record value to be different when encrypted twice, but it was the same")
	}
}
