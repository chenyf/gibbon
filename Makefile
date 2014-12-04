all: clean init gibbon agent tarball

agent:
	cd gibbonagent; GOOS=linux GOARCH=arm go build -o agent; go build -o agent.amd64

init:
	mkdir -p output
	rm -rf output/*

gibbon:
	cd gibbond && go build -o ../output/gibbond 

#gibbonapi:
#	cd gibbonapi && go build

tarball: init gibbon
	cp gibbond/control.sh output
	mkdir -p output/etc
	cp gibbond/etc/conf_product.json output/etc/conf.json
	cp gibbond/etc/log.xml output/etc/log.xml
	cp gibbond/etc/supervisord.conf output/etc/
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	go clean
	rm -rf gibbon gibbon.tgz output
	rm -f test/test
	rm -f gibbonagent/agent gibbonagent/agent.amd64
