.PHONY: all clean

all:
	go build -o hangman .

clean:
	rm -f hangman
