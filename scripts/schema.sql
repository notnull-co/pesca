CREATE TABLE IF NOT EXISTS Anzol (
    Id INTEGER PRIMARY KEY AUTOINCREMENT,
    RollbackTimeout INTEGER,
    RollbackStrategy INTEGER,
    RollbackEnabled BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS Isca (
    Id INTEGER PRIMARY KEY AUTOINCREMENT,
    AnzolId INTEGER NOT NULL,
    DeploymentActive BOOLEAN NOT NULL,
    DeploymentNamespace TEXT NOT NULL,
    DeploymentName TEXT NOT NULL,
    DeploymentContainerName TEXT NOT NULL,
    FOREIGN KEY (AnzolId) REFERENCES Anzol(Id),
    UNIQUE (DeploymentName, AnzolId, DeploymentNamespace, DeploymentContainerName)
);

CREATE TABLE IF NOT EXISTS ImageRevision (
    Id INTEGER PRIMARY KEY AUTOINCREMENT,
    IscaId INTEGER NOT NULL,
    PreviousImageRevisionId INTEGER,
    Version TEXT NOT NULL,
    Status INTEGER NOT NULL,
    CreatedAt DATETIME NOT NULL,
    UpdatedAt DATETIME NOT NULL,
    FOREIGN KEY (IscaId) REFERENCES Isca(Id),
    FOREIGN KEY (PreviousImageRevisionId) REFERENCES ImageRevision(Id)
);