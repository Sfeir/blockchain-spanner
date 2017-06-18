package main

import (
	"fmt"
	"log"
	"net/http"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1"

	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"golang.org/x/net/context"
	"os"
	"regexp"
	"crypto/sha1"
	"encoding/hex"
	"time"
	"strconv"
	"google.golang.org/api/iterator"
)


type block struct {
	BlockId    int64
	Message    string
	MyHash     string
	HashBefore string
	HashAfter  string
}

var blocksColumns = []string{"BlockId", "Message", "MyHash", "HashBefore", "HashAfter"}

func main() {
	ctx := context.Background()
	dataClient := createDataClient(ctx, getDatabaseName(ctx))
	http.HandleFunc("/", handle)
	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		handleCreate(w, r, ctx, dataClient)
	})

	http.HandleFunc("/write", func(w http.ResponseWriter, r *http.Request) {
		handleWrite(w, r, ctx, dataClient)
	})

	http.HandleFunc("/_ah/health", healthCheckHandler)

	log.Printf("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprint(w, "Hello world 2!")
}

func getDatabaseName(ctx context.Context) string {
	projectID := os.Getenv("GCLOUD_DATASET_ID")
	return "projects/" + projectID + "/instances/test-instance/databases/example-db"
}

func computeSha1(message string) string {
	h := sha1.New()
	h.Write([]byte(message))
	sha1_hash := hex.EncodeToString(h.Sum(nil))

	return sha1_hash
}

func writeMessage(blocksColumns []string, blockID int64, newMessage string, newMyHash string, newHashBefore string, newHashAfter string, client *spanner.Client, ctx context.Context) error {
	m := []*spanner.Mutation{
		spanner.InsertOrUpdate("Blocks", blocksColumns, []interface{}{blockID, newMessage, newMyHash, newHashBefore, newHashAfter}),
	}
	_, err := client.Apply(ctx, m)
	return err
}

func createDatabase(ctx context.Context, client *spanner.Client) error {

	db := getDatabaseName(ctx)
	adminClient := createAdminClient(ctx)

	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		return fmt.Errorf("Invalid database id %s", db)
	}
	op, err := adminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          matches[1],
		CreateStatement: "CREATE DATABASE `" + matches[2] + "`",
		ExtraStatements: []string{
			`CREATE TABLE Blocks (
				BlockId   INT64 NOT NULL,
				Message  STRING(1024),
				MyHash   STRING(1024),
				HashBefore STRING(1024),
				HashAfter  STRING(1024)
			) PRIMARY KEY (BlockId)`,
		},
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err == nil {
		log.Printf("Created database [%s]\n", db)
	}
	newMessage := "Block 0"
	newMyHash := computeSha1(newMessage)

	if err := writeMessage(blocksColumns, 1, newMessage, newMyHash, "", "", client, ctx); err != nil {
		return err
	}
	return nil
}
func handleCreate(w http.ResponseWriter, r *http.Request, ctx context.Context, dataClient *spanner.Client) {
	if r.URL.Path != "/create" {
		http.NotFound(w, r)
		return
	}

	err := createDatabase(ctx, dataClient)
	if err != nil {
		fmt.Fprint(w, err)
	}
	fmt.Fprint(w, "Create")
}

func findLastBlock(txn *spanner.ReadWriteTransaction, ctx context.Context) (block, error) {
	block := new(block)

	stmt := spanner.Statement{
		SQL: `select * FROM Blocks WHERE HashAfter = ""`}
	iter := txn.Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return *block, err
		}
		if err := row.ColumnByName("BlockId", &(block.BlockId)); err != nil {
			return *block, err
		}
		if err := row.ColumnByName("Message", &(block.Message)); err != nil {
			return *block, err
		}
		if err := row.ColumnByName("MyHash", &(block.MyHash)); err != nil {
			return *block, err
		}
		if err := row.ColumnByName("HashBefore", &(block.HashBefore)); err != nil {
			return *block, err
		}
		if err := row.ColumnByName("HashAfter", &(block.HashAfter)); err != nil {
			return *block, err
		}
	}

	return *block, nil
}


func writeWithTransaction(ctx context.Context, client *spanner.Client, newMessage string) error {

	_, err := client.ReadWriteTransaction(ctx, func(txn *spanner.ReadWriteTransaction) error {

		log.Printf(">>>>>>Begin Transaction")

		lastBlock, errFind := findLastBlock(txn, ctx)
		if errFind != nil {
			log.Printf(errFind.Error())
			return errFind
		}

		blockReadInTransaction := lastBlock

		timestamp := blockReadInTransaction.BlockId + 1 + time.Now().UnixNano()
		timestampString := strconv.FormatInt(timestamp, 10)

		newMyHash := computeSha1(timestampString + newMessage)
		newHashBefore := blockReadInTransaction.MyHash
		newHashAfter := ""

		log.Printf("previous Block %d, %s, %s, %s, %s", blockReadInTransaction.BlockId, blockReadInTransaction.Message, blockReadInTransaction.MyHash, blockReadInTransaction.HashBefore, newMyHash)
		log.Printf("new Block %d, %s, %s, %s, %s", blockReadInTransaction.BlockId+1, newMessage, newMyHash, newHashBefore, newHashAfter)

		txn.BufferWrite([]*spanner.Mutation{
			spanner.InsertOrUpdate("Blocks", blocksColumns, []interface{}{blockReadInTransaction.BlockId + 1, newMessage, newMyHash, newHashBefore, newHashAfter}),
			spanner.InsertOrUpdate("Blocks", blocksColumns, []interface{}{blockReadInTransaction.BlockId, blockReadInTransaction.Message, blockReadInTransaction.MyHash, blockReadInTransaction.HashBefore, newMyHash}),

		})

		log.Printf( ">>>>>>End Transaction")

		return nil
	})

	if err != nil {
		log.Printf(err.Error())
	}
	return err
}

func handleWrite(w http.ResponseWriter, r *http.Request, ctx context.Context, dataClient *spanner.Client) {
	if r.URL.Path != "/write" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprint(w, "Write")
	fmt.Fprint(w, "Write\n")
	message := r.URL.Query().Get("message")
	if message != "" {

		err := writeWithTransaction(ctx, dataClient, message)
		if err != nil {
			log.Printf(err.Error())
			fmt.Fprint(w, err)
		}

	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "ok")
}

func createDataClient(ctx context.Context, db string) *spanner.Client {
	dataBaseClient, err := spanner.NewClient(ctx, db)
	if err != nil {
		log.Printf(err.Error())
	}

	return dataBaseClient
}

func createAdminClient(ctx context.Context) *database.DatabaseAdminClient {
	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		log.Printf(err.Error())
	}

	return adminClient
}
