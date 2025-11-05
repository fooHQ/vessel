# Vessel

Foojank is a prototype of an agent for [foojank](https://github.com/foohq/foojank) command-and-control framework.

Foojank uses NATS as a command-and-control server. NATS, a widely used message broker, is commonly used in IoT and cloud systems to facilitate communication between distributed services. NATS offers a persistence layer known as JetStream, enabling it to store messages on the server even when the receiver is offline. Additionally, NATS provides an object store that can be utilized for storing files.

Foojank leverages the NATS features to offer:

* Asynchronous or real-time communication with Agents over TCP or WebSockets.
* Server-based storage for file sharing and data exfiltration.
* JWT-based authentication and authorization.
* Full observability.
* Extensibility.

Foojank is currently compatible only with our prototype agent, [Vessel](https://github.com/foohq/vessel). However, we plan to implement support for integrating custom agents into the framework in the future.

## Installation

TODO

## Usage

TODO

## License

This software is distributed under the Apache License Version 2.0 found in the [LICENSE](./LICENSE) file.
