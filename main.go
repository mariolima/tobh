package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

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
			for u := range logins {
				driver, err := neo4j.NewDriver("bolt://work.vm:7687", neo4j.BasicAuth("neo4j", "test", ""))
				if err != nil {
					log.Fatal(err)
					return
				}
				session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
				split := strings.Split(u, ":")
				userd := strings.ToUpper(split[0])
				if !strings.Contains(userd, "\\") {
					continue
				}
				domain := strings.Split(userd, "\\")[0]
				user := strings.Split(userd, "\\")[1] + "@" + domain
				pass := strings.Join(split[1:len(split)], "")
				stmt := "MATCH (H:User {name:$user}) SET H.password=$pass,H.owned=true RETURN H.name"
				params := map[string]interface{}{"user": user, "pass": pass}
				res, err := session.Run(stmt, params)
				session.Close()
				driver.Close()
				log.Debug(res)
				if err != nil {
					log.Fatal(err)
				}
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
