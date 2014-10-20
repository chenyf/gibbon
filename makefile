all: clean gibbon tarball

gibbon:
	go build

tarball: gibbon
	mkdir -p output
	rm -rf output/*
	cp gibbon output
	cp control.sh output
	cp -r etc output
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	go clean
	rm -rf gibbon gibbon.tgz output
