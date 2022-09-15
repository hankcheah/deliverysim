# DeliverySim
#### A food delivery system simulator

## How to Use
First, compile the project with

<code>
go build
</code>

then run

<code>
./deliverysim <path to Simulation Profile>
</code>

For example, use the following command to run a simulation using a sample profile that comes with the repositry.
<code>
./deliverysim profile/default-fifo.json
</code>

## Simulation Profile
A simulation profile is a JSON file that defines the parameters for a simulation run.
Several profiles are provided in the**profile/** directory.

For illustration, below is what a simulation profile looks like.

<code>
{
    "Name": "Matched - Default",
    "Desc": "Default profile for Matched courier dispatch strategy.",
    "Order": {
        "SourceFile": "./data/dispatch_orders.json",
        "Rate": 2,
        "Limit": -1
    },
    "Kitchen": {
        "Capacity": 100,
        "CounterSize": 30
    },
    "Courier": {
        "DispatchStrategy": "Matched",
        "PoolSize": 100,
        "TravelTime": [3, 15]
    },
    "DebugMode": false
}
</code>

### Supported parameters
Name - Name of the profile
Desc - A description of the profile

Order
* SourceFile - Path to the order data. A sample is provided at data/dispatch_orders.json
* Rate - Number of orders per second the system receives
* Limit - Total number of orders to issue. If set to -1, the System will use all available orders in the SourceFile

Kitchen
* Capacity - Number of orders the kitchen can process (cook) simultaneously.
* CounterSize - Max number of orders waiting for pickup on the counter before the kitchen has to stop cooking (out of counter space)

Courier
* PoolSize - Number of couriers
* DispatchStrategy (FIFO/Matched) - **FIFO** - first courier to arrive gets to pick up deliver first. **Matched** All orders are pre-matched with a courier.
* TravelTime (Integer array with 2 elements) - How long does it take for the courier to arrive? This parameter defines the range of courier travel time. Actual travel time is randomized.

DebugMode (true|false) - show debug message if set to true

## Tested with
Go 1.18.3
Linux
