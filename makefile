all: clean gibbon tarball

gibbon:
	go build

tarball: gibbon
	mkdir -p output
	rm -rf output/*
	cp gibbon output
	cp control.sh output
	cp etc/conf_product.json output/conf.json
	cp etc/log_product.xml output/log.xml
	cp etc/supervisord.conf output/
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	go clean
	rm -rf gibbon gibbon.tgz output
	rm -f test/test
