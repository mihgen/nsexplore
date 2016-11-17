## Explore and enter Linux namespaces
Only network namespaces are supported at the moment.
You can see ALL network namespaces and process pids associated with each.
It is different from "ip netns list" by ability to see unnamed namespaces.

#### Example usage
Make sure you are root before running this utility.

```bash
go build nsexplore.go

# List all network namespaces, and pids associated.
./nsexplore
```
Example output:
 NS NUMBER  PIDS
4026531957  8053,8077,10776,11790

To see what those pids are:

```bash
ps -p 8053,8077,10776,11790
```

Enter network namespace by number and run "ip addr":
```bash
./nsexplore -j 4026531957 ip addr
```
