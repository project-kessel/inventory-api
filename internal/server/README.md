# Server

The server package provides an http server that can be configured with serving certs and a CA for various
client cert validation options.

We should update it so the serving cert / private key along with the CAs used for client cert validation can
be rotated at runtime ("hitless certificate rotation").
