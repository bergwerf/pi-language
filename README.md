The PI language
===============
This is a simple programming language based on the Pi calculus. I wrote it to 
play with the ideas introduced by the Pi calculus myself without going through 
extensive literature. An interesting aspect of the Pi calculus is that all
logic is defined in a very localized manner; the only way to build larger 
programs is to figure out a way to let these parts communicate with each other 
independent of execution order.

I will try to build a fuzzer/prover to establish that a given PI program 
produces stable results. To work with input and output some special channels are 
added to the basic Pi calculus.

Hello, World!
-------------
```
! Print "Hello, World!\n" and exit.
<>stdout__H; <>stdout__e; <>stdout__l; <>stdout__l; <>stdout__o;
<>stdout_2C; <>stdout_20;
<>stdout__W; <>stdout__o; <>stdout__r; <>stdout__l; <>stdout__d;
<>stdout_21; <>stdout_0A.
```

Grammar
-------
The PI core language has the following grammar:

```
P,Q ::= +x;P | y<-x;P | y<<x;P | y->x;P | y->x. | PQ | (P)
```

All variable names must match the regular expression `[a-zA-Z0-9_]+`. There are
special IO channels to interact with input and output without introducing
data types. The IO channels are:
- `stdin_read` triggers a byte read.
- `stdin_[0-9A-F]{2}` triggers when a specific byte is read.
- `stdout_[0-9A-F]{2}` writes bytes to the standard output when triggered.
- `stdin__[a-zA-Z0-9]` and `stdout__[a-zA-Z0-9]` are aliases.
- `stdin_EOF` triggers when the stdin EOF is reached.
- `DEBUG` prints information about any channel sent to it.

I replaced the replication operator with a subscribe operator which will respawn
the subsequent process whenever a new element is received on the subscribed
channel. I believe this is more practical and easier to define. To prove that
Turing completeness is retained one could try to program a beta reduction
algorithm for the Lambda calculus in PI.

### Extensions
A supported syntactic sugar is the ability to use multiple arguments at once:
`x,y->v,w` is desugared to `x->v;x->w;y->v;y->w`. You can write line comments
after a `!` (inspired by Fortran, I believe the exclamation mark is perfect for
attracting the readers attention, as if the author is screaming at you to please
understand what is going on). To make working with multiple files more practical
there are two pre-processing directives:
- `#global: name` declares `name` to be a global channel.
- `#attach: file.pi` instructs the interpreter to include the program in
  `file.pi` and make its global channels available here.

Semantics
---------
Here is a list of scenarios I considered to determine an appropriate simulation
algorithm. This list is by no means complete, and I did not start from a formal
semantics because I want to start from a practical perspective (and save time).
In my simulation algorithm I try to allow future extensions for randomly
dropping or delaying processes and messages.

- `+x,y;x->y;z<-y.` Once a process sends it cannot receive what it sent later
  on. Hence here z is not equal to x, instead the process waits for the next
  message through y. It is guaranteed that z is always the message after x.
- `+x,y;(x->y.z<-y.)` Parallel processes in the same block are started
  simultaneously and can communicate with each other from the start. Thus here
  z is equal to x. A stricter rule is that receiving processes (subscribers) are
  started first such that they can always receive the first sent of any process
  in parallel. Without this rule a lot of constructs become near impossible.
- `x,y;(z<-y;v<-z.x->y;+v;v->x.)` When sending x to y the process cannot expect
  that the other process receives it right away. To make sure that the other
  process can receive v through x it has to wait for an acknowledgement.

Note: theoretical computer scientists may find it absurd I do not start from a
type theory or formal operational semantics. However I regard this project as a
puzzle for myself to discover what works and what doesn't. And I really do
dread reading long documents.

Using goroutines
----------------
An interesting exercise would be to make a PI interpreter that (ab)uses Go
routines and channels. The program processor (before simulation) would have to
determine when channels can be marked as dead (e.g. determine the information
needed such that the simulator can perform reference counting) such that they
can be closed (or else a multitude of open channels will accumulate). I am not
sure if it is possible to create a channel that can send arbitary typed channels
itself. Otherwise a central channel registry is needed.
