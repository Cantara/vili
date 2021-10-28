# vili

Vili atempts to add a layer of integrety to your application. It does this by managing your versions and how they get shown. By takin inn your new jar files and structuringthem vili contains everything relevant for one runtime in its own place. Vili also cleans up after itself and archives old versions as zips.
By executing two versions of your application and tailing their json logs can vili read out a metric on how good your new version is and through that decide if it sould be the running version based on REAL data from your users.
If vili decides a update is approtiate it will kill off the application that is in testing and start a new version marked running. Then vili will move traffic from the previous running application to the new one and then kill the old running application.
Vili will also do minor things like verify that you dont break an endpoint by only serving it on the old version and 404 ing on the new version.

Vili will also alway save all information needed to run any application by itself where you need it. You will therfore find that vili provides you with symlinks to the versions of you application that is running and being testet. Along with your base config file that is copied and edited for vilis needs.
