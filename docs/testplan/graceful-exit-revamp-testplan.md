# Graceful Exit Revamp

## Background

This testplan covers Graceful Exit Revamp
&nbsp;

| Test Scenario | Test Case                               | Description                                                                                                                                                                                                                       | Comments |   
|---------------|-----------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| Graceful Exit | Happy path                              | Perform GE on the node, satellite not send any new pieces to this node. Pieces on this node marked as "retrievable but unhealthy". After one month (with an appropriately high online score), the node will be considered exited. | Covered  |  
|               | GE on Disqualified Node                 | Make sure GE was not initiated for the disqualified node.                                                                                                                                                                         | Covered  |
|               | Double exit                             | Perform GE on the node and after receiving success message do it once again. Make sure node can not do it twice                                                                                                                   | Covered  |
|               | Low online score                        | Perform GE on node with less then 50% of score. Node should fail to GE                                                                                                                                                            | Covered  |
|               | Two many nodes call GE at the same time | We should transfer all the pieces to available nodes anyway. Example: start with 8 nodes(RS settings 2,3,4,4) and call GE on 4 nodes at the same time                                                                             |          |
|               | Audits                                  | SN should receive audits even if it perform GE at the moment                                                                                                                                                                      | Covered? |
|               | GE on Suspended node                    | Make sure GE was not initiated for the suspended node (Unknown audit errors).                                                                                                                                                     |          |
|               | GE started before feature deployment    | Node should stop transferring new pieces and should be treated by tne new rules.                                                                                                                                                  |          |