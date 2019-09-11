package storagenodedbtest

var v5 = Snapshots.Add(&MultiDBSnapshot{
	Version:   5,
	Databases: v4.Databases,
})
