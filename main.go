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
	// Number of concurrent neo4j query threads
	var concurrency int
	flag.IntVar(&concurrency, "c", 5, "number of threads")

	// Verbose flag for error msgs
	var verbose bool
	flag.BoolVar(&verbose, "v", true, "output errors to stderr")

	// Neo4j bolt URL
	var neo4jUrl string
	flag.StringVar(&neo4jUrl, "b", "bolt://localhost:7687", "neo4j endpoint (Default: bolt://localhost:7687)")

	// Neo4j bolt Username
	var neo4jUsername string
	flag.StringVar(&neo4jUsername, "u", "neo4j", "neo4j username (Default: neo4j)")

	// Neo4j bolt Password
	var neo4jPassword string
	flag.StringVar(&neo4jPassword, "p", "neo4j", "neo4j password (Default: neo4j)")

	flag.Parse()

	if verbose {
		log.SetLevel(log.TraceLevel)
	}

	logins := make(chan string)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			driver, err := neo4j.NewDriver(neo4jUrl, neo4j.BasicAuth(neo4jUsername, neo4jPassword, ""))
			if err != nil {
				log.Debug(err)
				log.Fatal(err)
				return
			}
			defer driver.Close()
			session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
			defer session.Close()
			for u := range logins {
				if err != nil {
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
				res, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
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
					log.Debug(err)
				}
				log.Debug(res.([]string))
			}
			wg.Done()
			return
		}()
	}

	// accept logins on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		logins <- sc.Text()
	}

	close(logins)

	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until all the workers have finished
	wg.Wait()
}
