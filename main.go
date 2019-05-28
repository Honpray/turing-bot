package main

import (
	"database/sql"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

var (
	token    string
	logPath  string
	dbSource string
	db       *sql.DB
	err      error
	stmt     *sql.Stmt
	logFile  *os.File
	um       map[string]int // user discord id: uid in user table
	pm       map[string]int // problem name: pid in problem table
)

func init() {
	token = "Bot NTgxOTkxNjkwODA2NjI0MjY3.XOsnkQ.CBvABPtPgErY7aEcdDbH8xSaUuE"
	logPath = "turing.log"
	dbSource = "./test.db"

	// Initialize map
	um = make(map[string]int)
	pm = make(map[string]int)

	// Log configurations
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(logFile)

	// Database initialization
	db, err = sql.Open("sqlite3", dbSource)
	if err != nil {
		log.Fatal(err)
	}
	dbInit()
	log.Println("db initialized")
}

func dbInit() {
	// Create User table
	stmt, _ = db.Prepare("CREATE TABLE IF NOT EXISTS user (uid INTEGER PRIMARY KEY, dcid TEXT, fname TEXT, lname TEXT, createts DATETIME);")
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}

	// Create Problem table
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS problem (pid INTEGER PRIMARY KEY, pname TEXT);")
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}

	// Create relationship table
	stmt, _ = db.Prepare("CREATE TABLE IF NOT EXISTS user_problem (upid INTEGER PRIMARY KEY, uid INTEGER, pid INTEGER, ts DATETIME, note TEXT);")
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}

	// select datetime(1559017037, 'unixepoch', 'localtime');
	// select strftime('%s', 'now');
}

func handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	user, content := m.Author, m.Content
	if content == "!test" {
		reply, err := s.ChannelMessageSend(m.ChannelID, "Test received!")
		if err != nil {
			log.Panic(err)
		}
		log.Printf("test @%v: %v", user.Username, content)
		log.Println("test reply:", reply.Content)
	} else if matched, err := regexp.MatchString(`^!solved [A-Z]+[0-9]+( -m ".+")?$`, content); matched && err == nil {
		slv := strings.SplitN(content, " ", 4)
		pname := slv[1]

		var uid, pid int
		var row *sql.Row

		// Search from um first
		if uid, ok := um[user.ID]; !ok {
			log.Println("uid not found in um")
			// Extract uid from database
			row = db.QueryRow("SELECT uid FROM user WHERE dcid=?", user.ID)
			err = row.Scan(&uid)

			// If not found, create new user entry and save to um
			if err == sql.ErrNoRows {
				log.Println(err)

				// Create user entry
				stmt, err = db.Prepare("INSERT INTO user (dcid, createts) VALUES (?, strftime('%s', 'now'));")
				if err != nil {
					log.Fatal(err)
				}
				_, err = stmt.Exec(user.ID)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("user created")

				// Extract again
				row = db.QueryRow("SELECT uid FROM user WHERE dcid=?", user.ID)
				err = row.Scan(&uid)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("get uid from db:", uid)
			}
			// Save to um
			um[user.ID] = uid
			log.Println("saved", uid)

		} else {
			log.Println("from um")
		}

		uid = um[user.ID]

		if uid == 0 {
			log.Fatal("uid cannot be 0")
		}

		if len(slv) == 2 {
			log.Println("solve parsed:", "pname", pname)

			// Insert current problem if not exist
			stmt, err = db.Prepare("INSERT INTO problem (pname) VALUES (?) EXCEPT SELECT pname FROM problem WHERE pname=?;")
			if err != nil {
				log.Fatal(err)
			}
			_, err = stmt.Exec(pname, pname)
			if err != nil {
				log.Fatal(err)
			}

			// Get pid
			row = db.QueryRow("SELECT pid FROM problem WHERE pname=?", pname)
			err = row.Scan(&pid)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("pid:", pid)

			// New up relationship entry
			stmt, err = db.Prepare("INSERT INTO user_problem (uid, pid, ts) VALUES (?, ?, strftime('%s', 'now'));")
			if err != nil {
				log.Fatal(err)
			}
			_, err = stmt.Exec(uid, pid)
			if err != nil {
				log.Fatal(err)
			}

		} else if len(slv) == 4 {
			note := slv[3]
			log.Println("solve parsed:", "pname", pname, "note:", note)
			stmt, err = db.Prepare("INSERT INTO problem (pname) VALUES (?) EXCEPT SELECT pname FROM problem WHERE pname=?;")
			if err != nil {
				log.Fatal(err)
			}
			_, err = stmt.Exec(pname, pname)
			if err != nil {
				log.Fatal(err)
			}

			// Get pid
			row = db.QueryRow("SELECT pid FROM problem WHERE pname=?", pname)
			err = row.Scan(&pid)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("pid:", pid)

			// New up relationship entry
			stmt, err = db.Prepare("INSERT INTO user_problem (uid, pid, ts, note) VALUES (?, ?, strftime('%s', 'now'), ?);")
			if err != nil {
				log.Fatal(err)
			}
			_, err = stmt.Exec(uid, pid, note)
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("solve @%v: %v", user, content)

		reply, err := s.ChannelMessageSend(m.ChannelID, "congrats!")
		if err != nil {
			log.Panic(err)
		}

		log.Println("solve reply:", reply.Content)
	}
}

func main() {
	// Create a new Discordgo session
	dg, err := discordgo.New(token)
	if err != nil {
		log.Fatal(err)
	}

	// Add command handler
	dg.AddHandler(handleCommand)

	// Update status on event Ready
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		s.UpdateStatus(0, "with Turing Machines")
		log.Println("update status")
	})

	// Create a new connection
	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Turing is running ...")

	// Wait until termination signal received
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	go func() {
		sig := <-sigs
		log.Println(sig)
		done <- true
	}()
	<-done

	dg.Close()
}
