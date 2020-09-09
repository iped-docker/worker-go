worker-go:
	docker build . -f Dockerfile-make -t worker-go
	docker run worker-go cat /go/bin/app > worker-go
	chmod +x worker-go

.PHONY: worker-go
