# Vili

[![Build Status](https://jenkins.entraos.io/buildStatus/icon?job=Cantara-Vili-Multipipeline%2Fmain)](https://jenkins.entraos.io/job/Cantara-Vili-Multipipeline/job/main/)[![Go Report Card](https://goreportcard.com/badge/github.com/cantara/vili)](https://goreportcard.com/report/github.com/cantara/vili)

Vili adds another layer of integrety and stability to your application. It does this by managing how new versions are deployed to users. Vili will take responsibility for running your application and forwarding traffic to it. Vili also listens to the logs of the applications it handles. This is to verify if a new version that is being tested is "stable" / "correct" enugh to be shown to users. This is a simple and allmost dropinn way of doing testing with live user data. One should allways consider that there might be some side effects if clusters are not propperly handled or you add incompatible datastructure updates in your persistent storage. Vili is also most likly noe a tool to use for major version updates.

/* This is things from the old readme that i might want to add inn somehow in the new

\By taking inn your new jar files and structuringthem vili contains everything relevant for one runtime in its own place. Vili also cleans up after itself and archives old versions as zips.
By executing two versions of your application and tailing their json logs can vili read out a metric on how good your new version is and through that decide if it sould be the running version based on REAL data from your users.
If vili decides a update is approtiate it will kill off the application that is in testing and start a new version marked running. Then vili will move traffic from the previous running application to the new one and then kill the old running application.
Vili will also do minor things like verify that you dont break an endpoint by only serving it on the old version and 404 ing on the new version.

Vili will also alway save all information needed to run any application by itself where you need it. You will therfore find that vili provides you with symlinks to the versions of you application that is running and being testet. Along with your base config file that is copied and edited for vilis needs.

*/

## TL;DR

Vili tests new versions of your servers with real data before showing them to your users

## Table of Contents

[TOC]



## Inspiration

Poka-yoke (ポカヨケ, [poka joke]) is a Japanese term that means "mistake-proofing" or "inadvertent error prevention". A poka-yoke is any mechanism in a process that helps an equipment operator avoid (yokeru) mistakes (poka) and defects by preventing, correcting, or drawing attention to human errors as they occur.

## Dictionary

* Base will refer to the directory in which vili has as its root.
* Running will refer to the current server, the version we trust and show users.
* Testing will refer to the server that is tested against the Running server and potentially becomes the next running.
* Server is a version of your service
* Servlet is an instance of a server
* Sermantic versioning is used to talk about different kinds of uppdates that vili does support or not. However vili is not reliant on semver.

## Important considerations for your applications

Vili will run multiple versions of your software in the same space and this brings with it some complications. But nothing good coding praccises won't help with.

1. You should only use vili when doing minor or patch updates.
2. The api should follow atleast the two first levels of restfullness or any other api standard that follows the same principles for methods. //TODO: verify what levels of the restfull standard actually talks about methigs or just change this to define that here again instead
3. You should handle versioning of your persistent data the same as your api. This means that if there is a change in how you handle your persistent data that is not backwards compatible with other versions of your software, you need a major version change.
4. How you handle versioning of your clustered data should be the same as your persistent data in point 3.
5. Propper handling of sigterm within a known max amount of time. It isn't optimal if this starts to take minutes.
6. A configurable host port

## Your responsibillities and Vilis responsibillities

### Yours

1. Familliarising yourselv with the [important considerations for your applications](##Important considerations for your applications).
2. Running and keeping vili alive.
3. If you kill vili hard, you also have to kill all the processes started by vili.
4. Downloading new server files that vili should use.
1. Cleaning up old server files that should no longer be used. Typical ways of hanlind this is keeping 4 versions as backup if you want to force an older version without rinning vili.
2. Not delete or edit any folders or files created by vili. Except the zip arcives.

### Vilis

1. Copy out any new server files when they are added to the **base** dir.
2. Create symlinks so it is easy to see what vili is doing and read rellevant logs.
3. Archive old servers
4. Create separations between each time a server is started so it is easy to see exactly what conditions the server ran under.
5. Delete old archives if the **base** folder exeeds a set amout of data.
6. Test and show new versions to your users when they are ready

## How to run Vili //Note to self, nothing beyond this point is uppdated

1. Prepare a **base** folder where Vili will run and all configs and new jar files will be located. 
2. Copy the tmp.env to a file named .env and fill inn.
   * port is whatever port you want vili to respond to
   * scheme is the scheme used to contact the servers you provide
   * endpoint is their base url / hostname / domain
   * port_range is the range of ports used to start and test your servers. You should have more than 5 port available
   * identifier is the base name of your server file. Without version information and without .jar. The prefix if you will.
   * log_dir is the dir vili logs to. If blank it logs to std
   * properties_file_name is **the** config file used for your applications. This will be copied to every instanve
   * port_identifier is the key in your properties file that corresponds to the port your server will run on
3. Setup a service like [Visuale's](https://github.com/Cantara/visuale) [semantic_update_service](https://github.com/Cantara/visuale/blob/master/scripts/semantic_update_service.sh) to downloade new verions into a base folder.
4. Start vili however you want.

## What Vili can give you

1. Vili can give you a way to test new versions of your software with real requests form your users without them noticing anything.
2. The confidence that when you deploy a new version there is a periode your software is proof tested with real data

## What vili can **not** give you

1. 100% confidence that all edge cases are tested
2. Sentralized syncronizy between all your servers and versions

## What Vili does

1. Vili starts by looking for file and folders matching the identifier given
2. 
   1. Then if Vili has been ran before it wil continue where it left
   2. If vili does not find the structure it expects then Vili will look for a .jar file and crete the appropriate structure for it. (Point 6 & 8 for more information)
3. Vili wil then start the running and or testing servers.
4. When a request comes inn.
   1. Vili will start by forwarding that request to the running server
   2. Then when the running server responds vili returns that response to the user
   3. A copy of the same request if then sent to the testing server if there is one
   4. Then the logs and statuse codes are checked against eachother to see if the testing server gets any new errors that the running server does not get.
   5. If the testing server has performed only a slight bit worse than the running server over a periode of time then it will be deployed. (The testing servers startup errors are counted and not the runnings startup errors. That is why it can have a few more warnings than the running server.)
5. When a deployment is triggered.
   1. Vili starts by killing the testing server
   2. Then starts a new running server of the same version the testing server was
   3. Then it migrates the new running server to be the current runnig server
   4. And last it kills the previos running server.
6. When a new .jar file with the identifier prefix is created in the base dir.
   1. Vili tries to create a new version directory for the file and move it in there.
   2. Then vili starts the new server as a testing server.
   3. Then it migrates to the new testing server and resets all testing data
   4. Lastly it kills the previous testing server.
7. After a migration
   1. Whenever a server is migrated, not simply started or killed Vili knows there are no other server that should run on that version.
   2. Vili then zips the whole version folder
   3. Then archives it into a archive folder located in the base folder
   4. And lastly deletes the version folder
8. Folder structure deffinitions
   1. Vilis base folder contains the following
      1. New server files
      2. Folders for each version of the server
      3. Propperties files
      4. Archive folder
      5. .env with Vili's config
      6. A symlink to the running version folder
      7. A symlink to the testing version folder
   2. Every version folder contains the following
      1. The jar file for the server
      2. And a directory for every running and testing server numbered based on number of startups
   3. Every instance folder contains the following
      1. A symlink to the servers jar file
      2. A copy of the properties file as it was when Vili started the server
      3. A file for stdOut
      4. A file for stdErr
      5. A folder named logs for logs
      6. And within the logs foder another folder named json for a json formated version of the logs. Expecting there to be one json object per line
   4. Archive folder contains the following
      1. Ziped version folders that is migrated away from
9. When there is starting to be a lack of free disk space //TODO
   1. Vili then deletes the oldest version from the archive folder
   2. Vili should also truncate and archive its own logs //TODO

