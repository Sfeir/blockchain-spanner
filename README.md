# blockchain-spanner

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
https://AppID.appspot.com/create
```

Check that the database is created :

```
gcloud beta spanner databases list --instance=test-instance
```

Check that the first block is created :

```
gcloud beta spanner databases execute-sql example-db --instance=test-instance --sql='SELECT * FROM Logs'
```

Add block to the chain :

```
https://AppId.appspot.com/write?message=<yourNewBlockMessage>
```

Check that the block has been added

```
gcloud beta spanner databases execute-sql example-db --instance=test-instance --sql='SELECT * FROM Logs'
```

