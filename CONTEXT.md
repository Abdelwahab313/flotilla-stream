# Flotilla

Real-time backbone of a simulated delivery fleet: drivers stream positions, the system tracks them, raises zone events and assigns orders. All traffic is synthetic.

## Fleet and movement

**Fleet**:
One delivery company using the platform; the tenant. Every driver, zone, order and dispatcher belongs to exactly one fleet.
_Avoid_: Tenant (in product language), company, organization, account

**Driver**:
A simulated courier with one device; seeded, never created at runtime.
_Avoid_: Courier, rider, agent

**Position**:
One location report from a driver's device, identified by (device session, device sequence).
_Avoid_: Ping, GPS point, location update

**Driver state**:
The derived current view of a driver: latest position, current zone, duty, activity, connectivity.
_Avoid_: Driver record, driver profile

**Duty**:
Whether the driver intends to work: on duty, on break, off duty, or suspended by a dispatcher.
_Avoid_: Availability, shift status

**Activity**:
What dispatch has the driver doing: idle, offered, or on route.
_Avoid_: Work status, busy

**Connectivity**:
How recently the driver's device reported: live, stale, or lost.
_Avoid_: Online, heartbeat status

**Driver status**:
The single label the console shows, combined from duty and activity (Available, Offered, On route, Finishing route, On break, Off duty, Suspended); connectivity is shown beside it, not in it.
_Avoid_: State (that is the whole driver state)

**City**:
A place where a fleet operates, with its own timezone; a fleet has one or more cities.
_Avoid_: Market, region, service area

**District**:
The part of a city the simulator drives in, with its own road network (Maadi, downtown Cairo); a simulator setting, not a dispatch boundary.
_Avoid_: Zone (that is the dispatch polygon), map, region

**Zone**:
A named polygon inside one city that dispatch and geofencing work on.
_Avoid_: Area, region, geofence (geofence is the mechanism, zone is the thing)

## Dispatch

**Order**:
A pickup point and a dropoff point, with a promise, to be served by one driver.
_Avoid_: Job, task, delivery

**Promise**:
The pickup-by and deliver-by times an order commits to.
_Avoid_: SLA, deadline, ETA (an ETA is the planner's estimate, not the commitment)

**Late**:
An order whose planned or actual stop time is past its promise.
_Avoid_: Delayed, overdue

**Waiting**:
An order in its zone's queue with no proposal, offer or assignment.
_Avoid_: Pending, unassigned, open

**Return to queue**:
A dispatcher action that takes an assigned, not-yet-picked-up order from its driver and puts it back to waiting.
_Avoid_: Unassign, reassign

**Offer**:
A time-limited proposal of one or more orders, with their stops placed in the driver's route, to one driver, who accepts, declines or lets it expire as a whole.
_Avoid_: Request, ping, batch

**Assignment**:
The confirmed binding of one order to one driver. A driver may hold many assignments at once.
_Avoid_: Match, booking

**Stop**:
One pickup or one dropoff of one order, at a point, with a planned time.
_Avoid_: Waypoint, task

**Route**:
A driver's ordered list of upcoming stops across all of its assignments.
_Avoid_: Trip, path, trail (a trail is where the driver has been)

**Planner**:
The pluggable part of dispatch that decides which orders go to which driver, the stop order, and the expected pickup and delivery times.
_Avoid_: Auto-routing, optimizer, router

**Assignment mode**:
A fleet setting: *auto* sends the planner's offers straight to drivers; *review* holds them as proposals for a dispatcher to approve.
_Avoid_: Auto-routing, feature flag

**Proposal**:
A planner output in review mode, awaiting a dispatcher's approval or rejection before it becomes an offer.
_Avoid_: Suggestion, draft offer

**Capacity**:
The number of concurrent orders a driver can carry; an input to the planner, not shown prominently in the console.
_Avoid_: Limit, max load

**Utilization**:
A driver's active assignments divided by its capacity, at a moment or averaged over an operations day.
_Avoid_: Load, occupancy

**Import job**:
One upload of many orders (file or API batch) into one city, reported row by row as created, skipped as duplicate, or rejected.
_Avoid_: Bulk upload, batch (an offer is not a batch either)

**External reference**:
The fleet's own id for an order; unique within a fleet and the key that makes re-imports harmless.
_Avoid_: Client id, order number

**API key**:
A fleet-scoped, revocable credential with scopes, used by a fleet's own systems to send and read orders.
_Avoid_: Token (tokens are short-lived and belong to people and devices), secret

## Console

**Console**:
The product UI the dispatcher runs the operations day from: live orders, drivers, assignments and the map; one design language, two modes.
_Avoid_: ERP, dashboard (reserved for metrics boards in the observability stack), frontend, app

**Operations day**:
One calendar day of operation in one city, in that city's local time; the console's default scope.
_Avoid_: Shift (that belongs to a driver), session

**Day profile**:
A simulator scenario that reproduces the shape of an operations day (ramp, lunch and dinner peaks, shift changes) with demand compressed and movement in real time.
_Avoid_: Soak (a load profile without a demand shape), day simulation

**Dispatcher**:
A fleet member who runs the operations day for one or more cities: watches drivers and orders and acts on them through dispatch commands.
_Avoid_: Operator (that is the platform role), user

**Fleet admin**:
A fleet member who can do everything a dispatcher can, plus change fleet settings, API keys and roles, across all the fleet's cities.
_Avoid_: Admin, owner, manager

**Platform operator**:
The person running the platform itself, outside any fleet; drives demos and load runs from the operator view.
_Avoid_: Admin, superuser, operator (alone, ambiguous)

**Actor**:
The person or system recorded on every action event: a dispatcher, a fleet admin, a driver's device, an API key, or dispatch itself.
_Avoid_: Author, user, source

**Command**:
A dispatcher's or API key's request to change something (cancel, return to queue, approve, set duty, claim…), identified by a command id and answered exactly once by its owner: the outcome, or a rejection that names why and, after a lost race, what won.
_Avoid_: Request (alone), edit, update, mutation

**Message**:
Text sent from a dispatcher to one driver, delivered through a channel (device push, SMS) and tracked as sent, delivered, read or failed.
_Avoid_: Notification, SMS (that is one channel), chat

**Channel**:
One way a message reaches a driver; pluggable behind one interface.
_Avoid_: Provider, transport

**Alert**:
A system-raised item that needs a dispatcher's attention, derived from an event; at most one open per kind and subject; acknowledged and resolved, never created by hand.
_Avoid_: Notification, alarm, incident

**Incident**:
A group of open alerts of one kind in one zone within a rolling window, handled as one item: claimed, acknowledged and resolved together.
_Avoid_: Alert group, case, ticket

**Severity**:
How loudly an alert interrupts: critical (toast and sound, cannot be muted), warning (badge), info (inbox only).
_Avoid_: Priority, level

**Claim**:
A dispatcher taking ownership of an incident so other dispatchers in the city see it is being handled.
_Avoid_: Assign (reserved for orders), take

**Watch**:
A dispatcher pinning a driver so every event for that driver is surfaced to them, whether or not it is on screen.
_Avoid_: Follow, focus, subscribe, track

**Public view**:
The read-only console mode served on the public demo URL: dispatcher screens with every action disabled.
_Avoid_: Guest mode, viewer

**Operator view**:
The local-only console mode for the platform operator: Day profile, order bursts, fault injection, fleet switching; no entity edit forms.
_Avoid_: Admin panel, control panel
