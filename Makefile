.PHONY: clean distclean test

clean:
	rm -rf logs 2600-0.txt

distclean: clean
	rm -f 2600-0.txt

test: 2600-0.txt
	mkdir logs
	go test

2600-0.txt:
	curl -LOC - https://gutenberg.org/files/2600/2600-0.txt
