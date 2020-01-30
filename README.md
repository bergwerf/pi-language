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
This demonstrates the use of unicode output channels to print "Hello, World!".
I view PI programs with font ligatures for arrows (I use FiraCode). I use
Notepad++ for editing because it has a reasonable interface for defining a
custom grammar for highlighting.

```
-- Hello world in PI: hello_world.pi
+x;
x->U0048; -- H
x->U0065; -- e
x->U006C; -- l
x->U006C; -- l
x->U006F; -- o
x->U002C; -- ,
x->U0020; --
x->U0057; -- W
x->U006F; -- o
x->U0072; -- r
x->U006C; -- l
x->U0064; -- d
x->U0021. -- !
```