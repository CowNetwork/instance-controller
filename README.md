Instance controller
===================

The instance controller takes care of creating `Pod`s corresponding to the spec defined by the `Instance` CRD.
An `Instance` wraps a `Pod` object and provides more a detailed `.Status` field. You can find the exact specification
in `api/<version>/instance_types.go`

License
=======

This code is licenced under AGPLv3 you can find a copy of the licence in the `LICENCE.md` file.
