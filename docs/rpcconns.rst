.. _rpc_conns:

RPCConns
========

**RPCConns** defines connection pools used by CGRateS components for inter-service communication. These pools enable services to interact both within a single CGRateS instance or across multiple instances.


Configuration Structure
-----------------------

Example configuration in the JSON file:

.. code-block:: json

    {
	"rpc_conns": {
		"conn1": {
			"strategy": "*first",
			"pool_size": 0,
			"conns": [{
				"address": "192.168.122.210:2012",
				"transport": "*json",
				"connect_attempts": 5,
				"reconnects": -1,
				"connect_timeout": "1s",
				"reply_timeout": "2s"
			}]
		}
	}
    }


Predefined Connection Pools
---------------------------

\*internal
    Direct in-process communication

\*birpc_internal
    Bidirectional in-process communication

\*localhost
    JSON-RPC connection to local cgr-engine on port 2012

\*bijson_localhost
    Bidirectional JSON-RPC connection to local cgr-engine on port 2014

Bidirectional Communication with SessionS
-----------------------------------------

Bidirectional connections are specifically designed and used for communication between agents and the :ref:`SessionS <sessions>` component. While agents can send requests using standard connections, bidirectional connections are necessary when SessionS needs to communicate back to the agents. 

When using bidirectional connections, SessionS maintains references to all connected agents, allowing it to send requests back to specific agents when needed (for example, to force disconnect a session or query active sessions).

.. note::
    Bidirectional connections (``*birpc_internal``, ``*birpc_json``, ``*birpc_gob``) are exclusively used for Agent-SessionS communication. All other service interactions use standard one-way connections.


Parameters
----------


Pool Parameters
^^^^^^^^^^^^^^^

Strategy
    Controls connection selection within the pool. Possible values:

    * ``*first``: Uses first available connection, fails over on network/timeout/missing service errors
    * ``*next``: Round-robin between connections with same failover as ``*first``
    * ``*random``: Random connection selection with same failover as ``*first``
    * ``*first_positive``: Tries connections in order until getting any successful response
    * ``*first_positive_async``: Async version of ``*first_positive``
    * ``*broadcast``: Sends to all connections, returns first successful response
    * ``*broadcast_sync``: Sends to all, waits for completion, logs errors that wouldn't trigger failover in ``*first``
    * ``*broadcast_async``: Sends to all without waiting for responses
    * ``*parallel``: Pool that creates and reuses connections up to a limit

.. note::
    Connections attempt failover to the next available connection in the pool on connection errors, timeouts, or service errors. Service errors (usually referring to "can't find service" errors) occur when attempting to reach services that are either temporarily unavailable during engine initialization or disabled in that particular instance.

PoolSize
    Sets the connection limit for ``*parallel`` strategy (0 means unlimited)


Connection Parameters
^^^^^^^^^^^^^^^^^^^^^

Address
    Network address, ``*internal``, or ``*birpc_internal``

Transport
    Protocol (``*json``, ``*gob``, ``*birpc_json``, ``*birpc_gob``, ``*http_jsonrpc``). When using ``*internal`` or ``*birpc_internal`` addresses, defaults to the address value. Otherwise defaults to ``*gob``.

ConnectAttempts
    Number of initial connection attempts

Reconnects
    Max number of reconnection attempts (-1 for infinite)

MaxReconnectInterval
    Maximum delay between reconnects

ConnectTimeout
    Connection timeout (e.g., "1s")

ReplyTimeout
    Response timeout (e.g., "2s")

TLS
    Enable TLS encryption

ClientKey
    Path to TLS client key file

ClientCertificate
    Path to TLS client certificate

CaCertificate
    Path to CA certificate


Transport Performance
---------------------

\*internal, \*birpc_internal
    In-process communication (by far the fastest)

\*gob, \*birpc_gob
    Binary protocol that provides better performance at the cost of being harder to troubleshoot

\*json, \*birpc_json
    Standard JSON protocol - slower but easier to debug since you can read the traffic

\*http_jsonrpc
    HTTP-based JSON-RPC protocol - slower than direct JSON-RPC due to HTTP overhead, but can integrate with web infrastructure and provides easy debugging through standard HTTP tools

.. note::
    While the "transport" parameter name is used in the configuration, it actually specifies the codec (*json, *gob) used for data encoding. All network connections use TCP, while internal ones skip networking completely.

Using Connection Pools
----------------------

Components reference connection pools through "_conns" configuration fields:

.. code-block:: json

    {
	"cdrs": {
		"enabled": true,
		"rals_conns": ["*internal"],
		"ees_conns": ["conn1"]
	}
    }

This configuration approach allows:

* Deploying services across single or multiple instances
* Selecting transports based on performance requirements
* Automatic failover between connections
