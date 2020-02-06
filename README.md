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
-- Print "Hello, World!\n" and exit.
->stdout__H,stdout__e,stdout__l,stdout__l,stdout__o;
->stdout_2C,stdout_20; -- ,<space>
->stdout__W,stdout__o,stdout__r,stdout__l,stdout__d;
->stdout_21,stdout_0A. -- !\n
```

Grammar
-------
The PI language has the following grammar:

```
P,Q ::= +x;P | y<-x;P | y<<x;P | y->x;P | y->x. | PQ | (P)
```

All variable names must match the regular expression `[a-zA-Z0-9_]+`. There are
special interface channels to interact with input and output without introducing
data types. The interface channels are:
- `stdin_read` triggers a byte read.
- `stdin_[0-9A-F]{2}` triggers when a specific byte is read.
- `stdout_[0-9A-F]{2}` writes bytes to the standard output when triggered.
- `stdin__[a-zA-Z0-9]` and `stdout__[a-zA-Z0-9]` are aliases.
- `stdin_EOF` triggers when the stdin EOF is reached.
- `debug` prints information about any channel sent to it.

I replaced the replication operator with a subscribe operator which will respawn
the subsequent process whenever a new element is received on the subscribed
channel. I believe this is more practical and easier to define. To prove that
Turing completeness is retained one could try to program a beta reduction
algorithm for the Lambda calculus in PI.

Extensions
----------
To make this language more practical there is support for comments (all text
after `--` until the next newline is ignored) and some pre-processing:
- `#global: .*\n` declares a global channel.
- `#attach: .*\n` imports all processes and global names from the given file.

Garbage collection
------------------
Ideas for garbage collection (memory optimization):
+ Collect channels if all existing sender processes have finished.
+ Collect processes if the trigger channel is cleaned up (this may included
  replicated processes).
  