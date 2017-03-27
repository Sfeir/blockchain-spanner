package main

import (
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1"
	"fmt"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
	"log"
	"net/http"

	"crypto/sha1"
	"encoding/hex"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"regexp"
)

const (
	databaseUrl = "projects/%s/instances/test-instance/database/example-db"
)

var (
	spannerClient *spanner.Client
)

func main() {
	sc, err := createDatabase()
	if err != nil {
		log.Fatal(err)
	}

	// init global spanner client
	spannerClient = sc

	// install handler
	http.HandleFunc("/write", writeData)
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

/*func getDatabaseName(ctx context.Context) string {
	return fmt.Sprintf(databaseUrl, appengine.AppID(ctx))
}*/

func getDatabaseName(ctx context.Context) string {
	return "projects/randommeetupgenerator/instances/test-instance/databases/example-db"
}

func createAdminClient(ctx context.Context) *database.DatabaseAdminClient {
	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return adminClient
}

func createDatabase() (*spanner.Client, error) {
	ctx := context.Background()

	db := getDatabaseName(ctx)
	adminClient := createAdminClient(ctx)

	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		return nil, errors.New("invalid database id" + db)
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
		return nil, err
	}
	if _, err := op.Wait(ctx); err == nil {
		log.Println("Created database " + db)
	}

	client, err := spanner.NewClient(ctx, db)
	if err != nil {
		log.Fatal(err)
	}

	newMessage := "Block 0"
	newMyHash := computeSha1(newMessage)
	newHashBefore := ""
	newHashAfter := ""

	blocksColumns := []string{"BlockId", "Message", "MyHash", "HashBefore", "HashAfter"}
	if err := writeMessage(blocksColumns, 1, newMessage, newMyHash, newHashBefore, newHashAfter, client, ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func computeSha1(message string) string {

	h := sha1.New()
	h.Write([]byte(message))
	sha1_hash := hex.EncodeToString(h.Sum(nil))

	return sha1_hash
}

func write(ctx context.Context, newMessage string) error {
	blocksColumns := []string{"blockId", "Message", "MyHash", "HashBefore", "HashAfter"}

	stmt := spanner.Statement{
		SQL: `select * FROM Blocks WHERE HashAfter = ""`}
	iter := spannerClient.Single().Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var blockIDPrevious int64
		var messagePrevious string
		var myHashPrevious string
		var hashBeforePrevious string
		var hashAfterPrevious string

		if err := row.ColumnByName("BlockId", &blockIDPrevious); err != nil {
			return err
		}
		if err := row.ColumnByName("Message", &messagePrevious); err != nil {
			return err
		}
		if err := row.ColumnByName("MyHash", &myHashPrevious); err != nil {
			return err
		}
		if err := row.ColumnByName("HashBefore", &hashBeforePrevious); err != nil {
			return err
		}
		if err := row.ColumnByName("HashAfter", &hashAfterPrevious); err != nil {
			return err
		}

		newMyHash := computeSha1(newMessage)
		newHashBefore := myHashPrevious
		newHashAfter := ""

		// add new message
		if err := writeMessage(blocksColumns, blockIDPrevious+1, newMessage, newMyHash, newHashBefore, newHashAfter, spannerClient, ctx); err != nil {
			return err
		}

		// update previous message
		if err := writeMessage(blocksColumns, blockIDPrevious, messagePrevious, myHashPrevious, hashBeforePrevious, newMyHash, spannerClient, ctx); err != nil {
			return err
		}

		return err

	}
	return nil

}
func writeMessage(blocksColumns []string, blockID int64, newMessage string, newMyHash string, newHashBefore string, newHashAfter string, client *spanner.Client, ctx context.Context) error {
	m := []*spanner.Mutation{
		spanner.InsertOrUpdate("Blocks", blocksColumns, []interface{}{blockID, newMessage, newMyHash, newHashBefore, newHashAfter}),
	}
	_, err := client.Apply(ctx, m)
	return err
}

func writeData(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Write")
	message := r.URL.Query().Get("message")
	if message != "" {
		c := context.Background()
		err := write(c, message)
		fmt.Fprint(w, err)
	}
}
