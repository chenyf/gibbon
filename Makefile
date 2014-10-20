all: clean init gibbon tarball

init:
	mkdir -p output
	rm -rf output/*

gibbon:
	cd gibbond && go build -o ../output/gibbond 

#gibbonapi:
#	cd gibbonapi && go build

tarball: init gibbon
	cp control.sh output
	cp -r etc output
	cp misc/setupenv.sh output
	tar -czf gibbon.tgz output

clean:
	go clean
	rm -rf gibbon gibbon.tgz output

