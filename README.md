# dissect

dissect is a small scripting/configuration language that can be used to "dissect"
binary data and extract field(s) of information they contain.

dissect is made of two main elements:

* a scripting/configuration language
* an interpreter

## example

```
block version (
  major: uint 16
  minor: uint 16
)

block record (
  seconds: uint 32
  micros:  uint 32
  incllen: uint 32
  origlen: uint 32
)

block ipv4 (
  # to be written
)

block ipv6 (
  # to be written
)

block tcp (
  # to be written
)

block udp (
  srcport: uint 16
  dstport: uint 16
  length:  uint 16
  chksum:  uint 16
)

data (
  magic: uint 32
  include version
  thiszone: int 32, enum (
    0 = "GMT"
  )
  sigfigs: uint 32
  snaplen: uint 32
  network: uint 32

  repeat [true] (
    include record
    pdata as bytes with incllen
  )
)
```

## syntax

### comments

### types and endianess

### top level elements

#### block

#### data

#### alias

#### define

#### declare

#### import

#### typedef

#### enum, polynomial, pointpair

### block elements

#### repeat

#### break

#### continue

#### include

#### match

#### if/else if/else

#### seek

#### peek

#### print

#### echo

#### let

#### del


### internal variables
