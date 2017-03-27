package main

import (
	"fmt"
	"regexp"
	"testing"
)

func TestGetDatabaseName(t *testing.T) {
	db := fmt.Sprintf(databaseUrl, "randommeetupgenerator.appspot.com")
	fmt.Println(db)

	matches := regexp.MustCompile("^(.*)/databases/(.*)$").FindStringSubmatch(db)
	fmt.Println(len(matches))
}
