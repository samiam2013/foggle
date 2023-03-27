# Foggle

## What is Foggle?

### Background
Foggle is short for fog-toggle. My home server is named fog, because it's the power of a large cloud server but at home. It's Dell R720 that used to run esxli for a big denim jeans brank you probably know well.

### The Problem
The problem with fog isn't actually it's fault. I use a residential grade UPS for both my mac mini and that server, with it getting the version with USB. I have apcusbd (APC's usb control daemon) set up and when the power goes out, the server gets the command to shut down. Works fine, but looking into how to get it to reboot seems like the road to madness.

### The Sane Person's Solution and my Rebuke
I seem to need to have the server reboot on power restored and then configure the UPS to power-cycle on restored power. I don't want to do that, because I have all of my networking equipment on the same UPS and a lot of the time when the power goes out my wifi keeps working for a while. I don't want to wait for the modem and router to boot if they're already working.

## My Solution
I have a 20ah lithum phosphate battery that just sits doing nothing and has for a year or longer. I also have a raspberry pi that I use to play with sensors (atmospheric data, movment sensor, etc.) I also have a relay and a USB power brick and so I thought, why not just use the pi to monitor the power and when it comes back on, turn on the server? 

### Hardware setup
The pin reading a signal is GPIO27, used for 3.3v digital read it's connected through the relay to ground on one of the ground pins. It's on the "normally closed" pin and the relay is triggered by power being available, so when the power is available it's sends a "high" signal. The relay is only in place because I did not want to squeeze a volage divider onto an already overcrowded breadboard.

### Software setup
When power is restored, the code runs through a series of checks and actions. 
* Check if an http port is responding, if it is, stop because the server is already on.
* Ping the IPMI hardware, in this case IDRAC, until it responds. 
* Once it responds, send a command to check the current state of the server, just to be sure.
* If the server is off, send a command to turn it on.

The server itself runs a discord bot that can alert me to power being restored by sending a boot message. I can also avoid the embarrassment of having a website but noticing it's down days after the power has been restored.

## Possible Improvements
* don't use this code, use a separate 12v source to drive the network equipment and just let the UPS power cycle to reboot the server.
* don't use the pi, use a microcontroller with network capability like an ESP32 that isn't 150$.
* don't use a relay, build a voltage divider to drop from 5v to 3.3v and use the pi's GPIO pins directly.
* don't use a relay, use a transistor instead
* don't electrically couple to anything except the USB power running the pi, use an optocoupler to read the signal indirectly instead
* get an enterprise grade UPS that can be last long enough or has a network interface to reboot the server.


