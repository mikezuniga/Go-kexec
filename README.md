# go-kexec
Function-as-a-Service (FaaS) based on Kubernetes Jobs

# How to build and run
In go-exec directory, run
```
go install
```

Change FileServerDir in gorilla-config.json to your go-kexec/html directory

Then, go to your bin, run
```
./Go-kexec -config=<path to config.json>
```

# Future work
1. Handlers should be more concurrent (goroutine)
2. Parallel execution for kexec
3. Plugable authentication
4. API Gateway bridge
5. Reverse proxy configuration
6. Integration test
7. Add web frontend using advanced web framework (eg AngularJS)
8. Tune DAL (mysql)
