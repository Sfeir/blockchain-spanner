# blockchain-spanner

The concept of blockchain is simple: a scalable and globally consistent database that maintains an growing list of ordered records.

Using [App Engine](https://cloud.google.com/appengine/) and [Spanner](https://cloud.google.com/spanner/)
it is possible to build a basic blockchain in 200 lines of code.

"Pull requests are welcome".

Create a spanner instance :

```
gcloud beta spanner instances create  test-instance  --config=regional-us-central1 --description="My Instance" --nodes=1
```

Check that the instance is created :

```
gcloud beta spanner instances list
```

Deploy the appengine code :

```
goapp deploy -application <AppID> -version <version>
```

Create the database :

```
curl https://AppID.appspot.com/create
```

Check that the database is created :

```
gcloud beta spanner databases list --instance=test-instance
```

Check that the first block is created :

```
gcloud beta spanner databases execute-sql example-db --instance=test-instance --sql='SELECT * FROM Blocks'
```

Add block to the chain :

```
curl https://AppId.appspot.com/write?message=<yourNewBlockMessage>
```

Check that the block has been added

```
gcloud beta spanner databases execute-sql example-db --instance=test-instance --sql='SELECT * FROM Blocks'
```


Delete spanner instance !!!!!

```
gcloud beta spanner instances delete test-instance
```