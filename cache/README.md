The cache directory may be used by plugins for temporary files between runs

It is important that the directory be *owned* by the user `circonus-agentd` will
run as (i.e. `nobody` on linux). This is required so that the plugins run as 
that user would be able to write to the directory.
