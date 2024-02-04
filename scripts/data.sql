-- INSERT INTO Anzol (Namespace, RollbackTimeout, RollbackStrategy, RollbackEnabled) VALUES ('exampleNamespace1', 60, 1, 1);
-- INSERT INTO Anzol (Namespace, RollbackTimeout, RollbackStrategy, RollbackEnabled) VALUES ('exampleNamespace2', 120, 2, 0);

-- INSERT INTO Isca (AnzolId, DeploymentName, DeploymentContainerName, RollbackTimeout, RollbackStrategy, RollbackEnabled) VALUES (1, 'Deployment1', 'Container1', 30, 1, 1);
-- INSERT INTO Isca (AnzolId, DeploymentName, DeploymentContainerName, RollbackTimeout, RollbackStrategy, RollbackEnabled) VALUES (2, 'Deployment2', 'Container2', 60, 2, 0);

-- INSERT INTO ImageRevision (IscaId, PreviousImageRevisionId, Version, Status, CreatedAt, UpdatedAt) VALUES (1, NULL, 'v1.0', 1, '2024-02-04 12:00:00', '2024-02-04 12:00:00');
-- INSERT INTO ImageRevision (IscaId, PreviousImageRevisionId, Version, Status, CreatedAt, UpdatedAt) VALUES (2, 1, 'v2.0', 2, '2024-02-04 13:00:00', '2024-02-04 13:00:00');

SELECT * FROM Anzol;
SELECT * FROM ImageRevision;
SELECT * FROM Isca;