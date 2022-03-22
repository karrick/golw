.PHONY: bench clean distclean hyperfine

bench: 2600-0.txt
	mkdir logs
	go test -bench=.

clean:
	rm -rf logs log-writer/log-writer lumberjack/lumberjack

distclean: clean
	rm -f 2600-0*.txt

hyperfine: lumberjack/lumberjack log-writer/log-writer 2600-0-100.txt
	mkdir logs
	hyperfine --style color --export-markdown BENCHMARKS.md --warmup 10 \
		'./lumberjack/lumberjack -dir logs 2600-0-10.txt' \
		'./log-writer/log-writer -dir logs 2600-0-10.txt'

lumberjack/lumberjack: lumberjack/main.go
	cd lumberjack && go build

log-writer/log-writer: log-writer/main.go
	cd log-writer && go build

2600-0.txt:
	curl -LOC - https://gutenberg.org/files/2600/2600-0.txt

2600-0-100.txt: 2600-0.txt
	for i in $$(seq 100); do cat 2600-0.txt >> $@ ; done
