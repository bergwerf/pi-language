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
This demonstrates the use of unicode channels to print "Hello, World!". The
Unicode channels enable a PI program to read and write all Unicode characters.
I view PI programs with font ligatures for arrows (I use FiraCode).

```
-- Print "Hello, World!\n" and exit.
+x;
x->stdout__H;
x->stdout__e;
x->stdout__l;
x->stdout__l;
x->stdout__o;
x->stdout_2C; -- ,
x->stdout_20; -- <space>
x->stdout__W;
x->stdout__o;
x->stdout__r;
x->stdout__l;
x->stdout__d;
x->stdout_21; -- !
x->stdout_0A. -- \n
```

Grammar
-------
The PI language has the following grammar:

```
P,Q ::= +x;P | y<-x;P | y<<x;P | y->x;P | y->x. | PQ | (P)
```

All variable names must match the regular expression `[a-zA-Z0-9_]+`. There are
special interface channels to interact with input and output without introducing
data types. These are:
- `stdin_read` triggers a byte read.
- `stdin_[0-9A-F]{2}` triggers when a specific byte is read.
- `stdout_[0-9A-F]{2}` writes bytes to the standard output when triggered.
- `stdin__[a-zA-Z0-9]` and `stdout__[a-zA-Z0-9]` are aliases.

Shadowing a special channel is allowed (but not recommended). I replaced the
replication operator with a subscribe operator which will respawn the subsequent
process whenever a new element is received on the subscribed channel. I believe
this is more practical and easier to define. To prove that Turing completeness
is retained one could try to program a beta reduction algorithm for the Lambda
calculus in PI.

Syntax shortcuts
----------------
I considered introducing some syntactic sugar or a way to define macros (for
example to send multiple channels to a function, or to send an new channel that
is only used as trigger), but I decided this goes against the initial goals of
keeping extreme minimalism and building a self-hosted compiler.

Garbage collection
------------------
Ideas for garbage collection (memory optimization):
+ Collect channels if all existing sender processes have finished.
+ Collect processes if the trigger channel is cleaned up (this may included
  replicated processes).

Self hosting interpreter
------------------------
Currently my goal is to program an interpreter for PI in PI. I don't know yet if
this is feasible.