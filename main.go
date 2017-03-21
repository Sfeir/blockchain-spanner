package spanner_blockchain

import (
	"fmt"
	"log"
	"net/http"
	//	"cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"

	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/appengine"
	"regexp"
	"crypto/sha1"
	"encoding/hex"
)

const databaseName string = "projects/deuxmilledollars/instances/test-instance/databases/example-db"

func init() {
	http.HandleFunc("/write", writeData)
	http.HandleFunc("/create", createDB)
}

func createDataClient(ctx context.Context, db string) *spanner.Client {

	dataClient, err := spanner.NewClient(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	return dataClient
}

func createAdminClient(ctx context.Context) *database.DatabaseAdminClient {
	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	return adminClient
}

func createDatabase(ctx context.Context, client *spanner.Client, w http.ResponseWriter) error {

	db := databaseName
	adminClient := createAdminClient(ctx)

	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	if matches == nil || len(matches) != 3 {
		return fmt.Errorf("Invalid database id %s", db)
	}
	op, err := adminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          matches[1],
		CreateStatement: "CREATE DATABASE `" + matches[2] + "`",
		ExtraStatements: []string{
			`CREATE TABLE Logs (
				LogId   INT64 NOT NULL,
				Message  STRING(1024),
				MyHash   STRING(1024),
				HashBefore STRING(1024),
				HashAfter  STRING(1024)
			) PRIMARY KEY (LogId)`,
		},
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err == nil {
		fmt.Fprintf(w, "Created database [%s]\n", db)
	}
	newMessage := "Block 0"
	newMyHash := computeSha1(newMessage)
	newHashBefore := ""
	newHashAfter := ""

	logsColumns := []string{"LogId", "Message", "MyHash", "HashBefore", "HashAfter"}
	if err := writeMessage(logsColumns, 1, newMessage, newMyHash, newHashBefore, newHashAfter, client, ctx); err != nil {
		return err
	}
	return nil
}

func computeSha1(message string) string {

	h := sha1.New()
	h.Write([]byte(message))
	sha1_hash := hex.EncodeToString(h.Sum(nil))

	return  sha1_hash
}

func write(ctx context.Context, client *spanner.Client, newMessage string) error {
	logsColumns := []string{"LogId", "Message", "MyHash", "HashBefore", "HashAfter"}

	stmt := spanner.Statement{
		SQL: `select * FROM Logs WHERE HashAfter = ""`}
	iter := client.Single().Query(ctx, stmt)
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var logIDPrevious int64
		var messagePrevious string
		var myHashPrevious string
		var hashBeforePrevious string
		var hashAfterPrevious string

		if err := row.ColumnByName("LogId", &logIDPrevious); err != nil {
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
		if err := writeMessage(logsColumns, logIDPrevious+1, newMessage, newMyHash, newHashBefore, newHashAfter, client, ctx); err != nil {
			return err
		}

		// update previous message
		if err := writeMessage(logsColumns, logIDPrevious, messagePrevious, myHashPrevious, hashBeforePrevious, newMyHash, client, ctx); err != nil {
			return err
		}

		return err

	}
	return nil

}
func writeMessage(logsColumns []string, logID int64, newMessage string, newMyHash string, newHashBefore string, newHashAfter string, client *spanner.Client, ctx context.Context) error {
	m := []*spanner.Mutation{
		spanner.InsertOrUpdate("Logs", logsColumns, []interface{}{logID, newMessage, newMyHash, newHashBefore, newHashAfter}),
	}
	_, err := client.Apply(ctx, m)
	return err
}

func writeData(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Write")
	message := r.URL.Query().Get("message")
	if(message != "") {
		c := appengine.NewContext(r)

		dataClient := createDataClient(c, databaseName)
		err := write(c, dataClient, message)
		fmt.Fprint(w, err)
	}
}

func createDB(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	dataClient := createDataClient(c, databaseName)
	err := createDatabase(c, dataClient, w)
	fmt.Fprint(w, err)
}