# go-kexec
Function-as-a-Service (FaaS) based on Kubernetes Jobs

# How to build and run
In Go-exec directory, run
```
go install
```

Change FileServerDir in config.json to your Go-kexec/static directory

Then, go to your bin, run
```
./Go-kexec -config=<path to config.json>
```

# Future work
1. Handlers should be more concurrent (goroutine)
2. Parallel execution for kexec
3. Plugable authentication
5. Reverse proxy configuration
6. Integration test
8. Tune DAL (mysql)
