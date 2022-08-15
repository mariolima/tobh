package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"

	log "github.com/sirupsen/logrus"
)

func main() {
	// verbose flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 5, "concurrency")

	var verbose bool
	flag.BoolVar(&verbose, "v", true, "output errors to stderr")

	flag.Parse()

	if verbose {
		log.SetLevel(log.TraceLevel)
	}

	logins := make(chan string)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			driver, err := neo4j.NewDriver("bolt://work.vm:7687", neo4j.BasicAuth("neo4j", "test", ""))
			defer driver.Close()
			session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
			defer session.Close()
			for u := range logins {
				if err != nil {
					log.Fatal(err)
					return
				}
				split := strings.Split(u, ":")
				userd := strings.ToUpper(split[0])
				if !strings.Contains(userd, "\\") {
					continue
				}
				domain := strings.Split(userd, "\\")[0]
				user := strings.Split(userd, "\\")[1] + "@" + domain
				pass := strings.Join(split[1:len(split)], "")
				res, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
					var list []string

					stmt := "MATCH (H:User {name:$user}) SET H.password=$pass,H.owned=true RETURN H.name"
					params := map[string]interface{}{"user": user, "pass": pass}
					result, err := tx.Run(stmt, params)
					if err != nil {
						return nil, err
					}

					for result.Next() {
						list = append(list, result.Record().Values[0].(string))
					}
					if err = result.Err(); err != nil {
						return nil, err
					}

					return list, nil
				}, neo4j.WithTxTimeout(3*time.Second))
				if err != nil {
					log.Fatal(err)
				}
				log.Debug(res.([]string))
			}
			wg.Done()
			return
		}()
	}

	// SETUP client

	// accept logins on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		logins <- sc.Text()
	}

	close(logins)

	// check there were no errors reading stdin (unlikely)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until all the workers have finished
	wg.Wait()
}
