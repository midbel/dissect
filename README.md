# dissect

dissect is a small scripting/configuration language that can be used to "dissect"
binary data and extract field(s) of information they contain.

dissect is made of two main elements:

* a scripting/configuration language
* an interpreter

## Example

the example below shows how to use dissect to "parse" structure found into pcap file.
This example only shows how to get and print the packet headers added before each
captured packet. To dissect data further the blocks ipv4/ipv6/tcp should be extended
and the "data" block should also be extended.

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

    echo "packet #%[$Iter] (%[$Pos/8])"

    print raw as csv with seconds micros incllen origlen
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

#### include

#### typedef

#### enum, polynomial, pointpair

### block elements

#### repeat

##### break

##### continue

#### include

#### match

#### if/else if/else

#### seek

#### peek

#### print

#### echo

#### copy

#### let

#### del


### internal variables

#### Iter

#### Loop

#### Num

#### Pos

#### Size

#### File

#### Block

#### Path
