all:
	go build

tarball:
	go build
	mkdir -p output
	rm -rf output/*
	cp gibbon output
	cp control.sh output
	cp -r etc output
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	rm -rf gibbon gibbon.tgz output
