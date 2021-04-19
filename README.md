# Olivia's JobCoin Mixer

## Structure:

<img width="1075" alt="Screen Shot 2021-04-18 at 9 09 52 PM" src="https://user-images.githubusercontent.com/17032867/115168881-71cb1e00-a08a-11eb-97d9-6c41f2a5321f.png">

## How to run:

1. `make all`
2. `./bin/mixer`

## TODOS
Please see comments in the code where I note potential issues/assumptions I made/future enhancements that could be made. The mixer service I created is running locally for the time being. I tried to break the problem up into chunks that lend themselves to containerization -- the mixer service, the client service, the dispatcher, etc. As of right now, I have no external storage system/database set up. The data saved during mixing dies along with the mixer service.

## Tests

I provided three testing files `clientlib/lib_test.go`, `mixerlib/lib_test.go` `internal/dispatcher_test.go` for test coverage of some of the major functions used
