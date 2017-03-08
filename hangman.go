package main

import (
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	rand.Seed(int64(time.Now().Nanosecond()))
	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		panic("Failed to listen")
	}

	dict, err := ioutil.ReadFile("/usr/share/dict/words")
	if err != nil {
		panic(err)
	}

	words := strings.Split(string(dict), "\n")

	// Oh no, people can MITM our hangman sessions.
	// TODO: this really should be an argument
	privkey, err := ssh.ParsePrivateKey([]byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEApnzAmdwodU470yUHEbZFYzq5GvbmSjDj1DCMiR4KiNIbKuax
wd7xkwcB4Ff28Z/H4shcL6XJNqo3f3XMDpmQDv8f6xjhfZl+3Do0BtvbYakJLrIh
zKinHCf95pGX8O5fIft8NlEM8P6Mvx+UmhtHccwdJ5usXbaNNvxLwDNYllhP4DU4
avDztygclZzc5Y3+f0DzciEsSNbbJu7tsPcarB/aWovJk7owFM4GakIW90L0DZiJ
7BrmpbFKswf6jk+jcLw/DyoTn+FZF5iq9Nf7AulgKHcRMb4vDAb7RcV0FxG4k3b+
kIhCEj1+RVt49esjCtNi/3p5Mpsoi2qkde/1DwIDAQABAoIBADdWKdI6Gfx7h2jz
2ripY8DKqPHsdLjeLSu/A0ckBA5b/4mv6g9tYdAjuRzvP/YpzI91VybDLPENfKrR
5YRIyFgjtmE3AOP1W/QpKFfLRczdGV869/8FY535MOwtIlqDcH1kEHIhWHLVuMRh
48uhG4sYc+xRUuZHIgLPswHsTxqRMKPOlxqWfhwm01AAHIdUFjF2qDcHV3rZZ6Rk
+kmC9+nJIun4R61T3y/BaLy67JXd+vNe01RBosoAKntFKLDdXIFHNbQUEbpflKKV
Zh983LC5tkIWzVdgeqI6s8A8KL/W7xl6CKa1MdtDX2J4Mu5tAoKWhe5NnV4TgOXN
sZaHyCECgYEA0N0UJ1bEeHTA/7nrXpj1pZMtruE70MBtYwTAr8QAi4vpk5l5MXC0
PpUTmObTDgDlide1p+JcHIcjBr1/URF8b9TTCkJUepT50sFfC8UgO2H0+oYZ/JHV
bQW99b9DUcSTzB9bPts8geNR8PRaWCbi6kD1UFipGNpWeUuTjE07+Z8CgYEAzA9r
WpxaKomBPo3UWCtsRSj9NmuBX/kVHgr5bk3Vjb3wV5IJp6qLSlzCGKMEHusHb/QH
cqYiIYyCuBQMB5B5Zea6tuT/7owI7LWDH8H5oTf6giXJ5tLqjk5hrZh3hZ4HDM2B
psLZHMAp9fXq11N7Je5MldInZSq3zGo5R7TDLpECgYBko0HEusAsMkWUcqcAi79B
KquGaiyCJ1YNsgqOJmkE1EOVxQtqUR3oUbiBoibrxm7TfkafQCiV7l9oLgKcs0o8
MFYHyXIfJo1Bib8cUr13H3oFBHydAD/QKkYflSsAyTFV134FyWlcRIYFDhhCBWqs
6OPK9Q11Vi74fvv3THerjQKBgQC2hRg13OG9aG6FACEtl/Fozufl1DTNHiLoU1KV
eeMIvBqMiWA/awjZw6wMk0rtSLXHSlObFcQzk9WcRroXzf73yW+6hYvx16ln6FbX
gg4BSOu1m41C++a20J8HwfcuOZH+vpY4DdJap/sgOA0M+muNGt+/plB8acAqg9ym
KaXnMQKBgQDJJUOq+SgO9TxL3t971nkmRK1/wnQCnolYLjeCNXQDf4ZZN0w8/4lN
7Uwtf78DmZuoY1FkI7R1xRtn8hUccUUghMCzTPaNKT010vjvUWxmRlxv5/KUWgFE
M7e+7EosDONRsAxrPE94CNGal1TF4v7Ejks/9xIP2MfAAYD1q4Jk2g==
-----END RSA PRIVATE KEY-----
			`))
	if err != nil {
		panic(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			logrus.Warnf("Error accepting connection: %v", err)
			continue
		}
		go func(conn net.Conn) {
			serverConf := &ssh.ServerConfig{NoClientAuth: true}
			serverConf.AddHostKey(privkey)
			_, nchans, reqchan, err := ssh.NewServerConn(conn, serverConf)
			if err != nil {
				logrus.Warnf("Error getting ssh conn: %v", err)
				conn.Close()
				return
			}

			word := ""
			for word == "" {
				word = words[rand.Intn(len(words))]
				if !regexp.MustCompile("^[a-z]+$").MatchString(word) {
					word = ""
				}
			}
			logrus.Info("Picked: " + word)

			guessedLetters := []string{}

			strikes := 0

			go ssh.DiscardRequests(reqchan)

			for nchan := range nchans {
				if nchan.ChannelType() != "session" {
					nchan.Reject(ssh.UnknownChannelType, "get fucked")
					continue
				}

				session, reqs, err := nchan.Accept()
				if err != nil {
					logrus.Errorf("Could not accept session: %v", err)
				}

				go func(in <-chan *ssh.Request) {
					for req := range in {
						ok := false
						switch req.Type {
						case "shell":
							ok = true
						case "pty-req":
							ok = true
						}
						req.Reply(ok, nil)
					}
				}(reqs)
				term := terminal.NewTerminal(session, "")

				defer session.Close()
				for {
					totalGuesses := writeHangman(term, strikes, word, guessedLetters)
					if totalGuesses == word {
						term.Write([]byte("YOU GOT IT!\r\n"))
						session.Close()
						conn.Close()
						return
					}

					if strikes == 7 {
						term.Write([]byte("Sorry bro, YOU LOST. Get out of here\r\n\r\n... And next time remember the word " + word + " you oaf\r\n"))
						session.Close()
						conn.Close()
						return
					}

					term.Write([]byte("\r\nGuess a letter\r\n"))
					var l string
					for l, err = term.ReadLine(); len(l) != 1; l, err = term.ReadLine() {
						if err == io.EOF {
							session.Close()
							conn.Close()
							return
						}
						if err != nil {
							logrus.Errorf("error getting line: %v", err)
						}
						if strings.HasPrefix(l, "guess: ") {
							if l[len("guess: "):] == word {
								term.Write([]byte("YOU GOT IT!\r\n"))
								session.Close()
								conn.Close()
								return
							} else if strings.HasPrefix(l, "LET ME OUT") {
								term.Write([]byte("GOODBYE ;_;\r\n"))
								session.Close()
								conn.Close()
								return
							} else {
								strikes++
								term.Write([]byte("Sorry, that wasn't the word :(.. go again!\r\n"))
							}
						} else {
							term.Write([]byte("One letter only pls\r\n"))
						}
					}
					guessedLetters = append(guessedLetters, l)

					if !strings.Contains(word, l) {
						strikes++
					}
				}
			}
		}(conn)
	}
}

func writeHangman(w io.Writer, strikes int, word string, guesses []string) string {
	hangmanParts := []string{"o", "/", "|", "\\", "|", "/", "\\"}

	for i := 6; i >= strikes; i-- {
		hangmanParts[i] = " "
	}

	wordMatchingGuesses := strings.Repeat("_", len(word))
	for _, guess := range guesses {
		for i, char := range word {
			if guess == string(char) {
				// This is dumb
				wordMatchingGuesses = wordMatchingGuesses[0:i] + string(char) + wordMatchingGuesses[i+1:]
			}
		}
	}

	w.Write([]byte(`*** HANGMAN ***` + "\r\n" +
		` /---\ ` + "\r\n" +
		` |  ` + hangmanParts[0] + "\r\n" +
		` | ` + hangmanParts[1] + hangmanParts[2] + hangmanParts[3] + "\r\n" +
		` |  ` + hangmanParts[4] + "\r\n" +
		` | ` + hangmanParts[5] + " " + hangmanParts[6] + "\r\n" +
		` ___________   ` + wordMatchingGuesses + "\r\n\r\n" +

		strings.Join(guesses, " ") + "\r\n"))

	return wordMatchingGuesses
}
