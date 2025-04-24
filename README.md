# Server-Client Hierarchy Lifecycle Management Pattern

## Overview

This is a pattern for organising struct instances (herein called "Nodes") into hierarchies where each parent is a "server" and its children are "clients".
Nodes are supposed to run some continuous tasks.
Typically, clients will feed on some data provided by their servers, which is why they are so called (instead of the more generic "children" and "parents").

The second purpose of arranging Nodes into a hierarchy is to facilitate execution control: asking a Node to stop will ensure that
all its descendants are gracefully shut down before it gracefully shuts down itself.

The Node struct is supposed to be embedded in other, arbitrary structs.

At a high abstract level, a Node has a data input, an input queue, an output queue and an output.
This allows for clear concern separation and flexible management of Node lifecycle.
The input can be either internal (i.e. the Node procures the data itself), or supplied by its server Node.
The output can also be either internal (i.e. the Node pipes the data out of the hierarchy), or passed down to its clients.

Nodes can be started and stopped, although not manually: it happens automatically depending on their principal role and whether they have clients/servers.
Although a Node can be a client (attached to a server) and a server (i.e. having its own clients) at the same time,
it always has one designated principal role — either client or server, which defines when it starts/stops:

- a Node that is principally a server starts when it gets its first client attached, and stops when its last client is detached;

- a Node that is principally a client starts when it gets attached to a server, and stops when detached.
 
Before a Node is detached from its server, it detaches its own clients first, so that the lowest leaf clients finish detaching first, then their servers,
and so all the way up the hierarchy to the servers which started this cascade detaching.

When started, a Node can optionally run its own goroutine. This will gracefully stop on stopping the Node.

## Originating use-case

This pattern emerged as a convenient abstraction for the following real use-case:

The apex Node is designated to orchestrate an array of CCTV cameras, the process of saving their video streams to disk, and handle streaming of the videos to web clients.

This apex CCTV Node is principally a server. Its clients are Camera Nodes which receive configuration as to
which cameras (IP adresses) to connect to and what to do with the streams obtained from those cameras.

The Camera Nodes are principally servers too, although they are clients to the CCTV Node.

As each Camera may have more than one stream, the clients of Cameras are Stream Nodes. Each Stream Node pulls a specific video stream from the Camera,
and makes it available for its own clients such as Disk Writer and Web Broadcast Nodes.

The Stream Nodes are still principally servers — they run only if they have a Disk Writer or Web Broadcaster attached (otherwise there is no need to pull data from the camera and use network bandwidth).
The Disk Writer Node is principally a client: it starts doing its job immediately upon attaching to a Stream, and finishes it immediately upon detaching.
The Web Broadcaster Node is still principally a server: it gets its own client Nodes — Browser Sessions which are principally clients.

In this scenario Streams, Disk Writers, Web Broadcasters and Browser Sessions run their own goroutines because they've got heavy
I/O to do which must not block the main program execution.

Conversely, the apex CCTV node and Cameras do not run their own goroutines. They only initially create and attach their clients, but otherwise do not do anything.

| Node example | Principal type | Input | Output | Role |
| --- | --- | --- | --- | --- |
| CCTV Apex | Server | None | None | Sets up and waits for Camera to stop having any running Streams |
| Camera | Server | None | None | Client of CCTV. Sets up and waits for Streams to stop/detach |
| Stream | Server | Video stream from physical camera | Video stream for client Nodes | Client of Camera. Can detach its own clients when interrupted |
| Disk Writer | Client | Video stream from Stream | Video files on disk | Client of Stream. Starts writing when attached, stops when detached by it (can’t stop by itself) |
| Web Broadcaster | Server | Video stream from Stream | Video stream for Browser Session | Client of Stream. Stops (detaches from Stream) when there are no Browser Sessions attached |
| Browser Session | Client | Video stream from Web Broadcaster | Video stream for connected web browser | Client of Web Broadcaster. Stops (detaches from Web Broadcaster) on browser stopping/pausing video playback |


For an example implementation see [github.com/greendrake/cctv](https://github.com/greendrake/cctv).
