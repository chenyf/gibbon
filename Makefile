all: clean init gibbon agent tarball

agent:
	cd gibbonagent; GOOS=linux GOARCH=arm go build

init:
	mkdir -p output
	rm -rf output/*

gibbon:
	cd gibbond && go build -o ../output/gibbond 

#gibbonapi:
#	cd gibbonapi && go build

tarball: init gibbon
	cp control.sh output
	mkdir -p output/etc
	cp etc/conf_product.json output/etc/conf.json
	cp etc/log_product.xml output/etc/log.xml
	cp etc/supervisord.conf output/etc/
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	go clean
	rm -rf gibbon gibbon.tgz output
	rm -f test/test
