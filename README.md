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
```bash
 NS NUMBER  PIDS
4026531957  8053,8077,10776,11790
```

To see what those pids are:

```bash
ps -fp 8053,8077,10776,11790
```

Enter network namespace by number and run "ip addr":
```bash
./nsexplore -j 4026531957 ip addr
```

#### Docker containers

If a process is running withing a Docker container, it takes a couple of steps to get a name of this container.

1. Get process pid
```bash
./nsexplore
...
4026532647  13240,13486
```

2. Get PPID of process you are interested in
```bash
ps -fp 13240,13486
UID        PID  PPID  C STIME TTY          TIME CMD
root     13240 13223  0 Nov19 ?        00:00:00 /pause
nobody   13486 13470  0 Nov19 ?        00:03:38 dnsmasq -k -7 /etc/dnsmasq.d
```
Note PPID of dnsmasq is 13470.

3. Check PID name of that process
```bash
ps -fp 13470
UID        PID  PPID  C STIME TTY          TIME CMD
root     13470  6135  0 Nov19 ?        00:00:00 docker-containerd-shim 5edae692e6cc032f884...
```
Note sha of container in the output.

4. Get container name from docker ps output
```bash
docker ps | grep 5edae692e6
```
