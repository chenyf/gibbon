all: clean gibbon tarball

gibbon:
	go build

#test:
#	cd test; go build

tarball: gibbon
	mkdir -p output
	rm -rf output/*
	cp gibbon output
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
