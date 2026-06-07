## Summary
An `AdHighErrorRate` alert fired for the `ad` service, indicating its gRPC error ratio exceeded 5%. The automated remediator successfully intervened by disabling the `adFailure` feature flag, which is designed to inject faults into the `ad` service for testing purposes. The service has since recovered, with the current error ratio well below the alert threshold.

## Likely root cause
The likely root cause was the `adFailure` feature flag being enabled. In the OmniObserve architecture, faults are intentionally injected into services via flagd feature flags for testing. The `adFailure` flag, when active, causes the `ad` service's `GetAds` gRPC method to throw errors, directly leading to an elevated error ratio. The remediator's action of disabling this flag is consistent with this being the intended fault injection mechanism.

## Evidence considered
*   **Alert:** `AdHighErrorRate` for service `ad`, indicating the gRPC error ratio was above 5%.
*   **Automated action:** The remediator disabled the `adFailure` flag. This action is specifically designed to mitigate test-injected faults in the `ad` service.
*   **System architecture:** OmniObserve uses flagd feature flags (e.g., `adFailure`) to inject faults for testing. A firing alert often traces back to an enabled fault flag.
*   **Codebase knowledge (Ad Service):** The `GetAds` method in `oteldemo.AdService.java` explicitly checks the `adFailure` flag. If `on`, it throws a `StatusRuntimeException` (Status.UNAVAILABLE/INTERNAL), directly causing gRPC errors and increasing `rpc_grpc_status_code`.
*   **Prometheus metrics:** The current gRPC error ratio (5m) is 0.0083 (0.83%), which is significantly below the 5% alert threshold. This indicates that the service has recovered *after* the remediator's action, supporting the conclusion that the `adFailure` flag was the cause. The alert fired when the ratio *was* above 5%.

## Proposed remediation
The most direct fix was the automated action already taken by the remediator: disabling the `adFailure` flag. This incident was triggered by a test-injected feature flag, not a genuine code or configuration defect.

## Code-level fix
The remediator's automated action (disabling the `adFailure` flag) already stopped the bleeding by preventing the fault injection. However, for a durable code-level fix that aligns with the "Correct remediation" guidance for this service, the `AdService` should be modified to handle errors gracefully rather than propagating them.

The relevant file is `src/ad/src/main/java/oteldemo/AdService.java`.
The `getAds` method currently calls `checkAdFailure`, which throws a `StatusRuntimeException` if the `adFailure` flag is enabled.

The proposed change is to modify the `getAds` method to catch the exception thrown by `checkAdFailure` (or any downstream error) and return an empty `AdResponse` or a cached default, rather than re-throwing the gRPC error. This ensures that ads, being a non-critical page element, fail soft without impacting the overall page health or burning error budget.

**Sketch of the change in `AdService.java`:**

```java
// Inside AdService.java

public class AdService extends AdServiceGrpc.AdServiceImplBase {

    // ... existing code ...

    @Override
    public void getAds(AdRequest req, StreamObserver<AdResponse> responseObserver) {
        try {
            // Original fault injection path
            checkAdFailure(); // This method throws StatusRuntimeException if adFailure flag is on

            // ... existing logic to fetch and return ads ...
            // If checkAdFailure() doesn't throw, proceed as normal
            List<Ad> ads = fetchAds(req.getContextKeysList());
            AdResponse reply = AdResponse.newBuilder().addAllAds(ads).build();
            responseObserver.onNext(reply);
            responseObserver.onCompleted();

        } catch (StatusRuntimeException e) {
            // Catch the fault injection error or any other downstream gRPC error
            // Log the error for debugging, but do not propagate it to the frontend
            System.err.println("AdService.getAds failed due to: " + e.getMessage() + ". Returning empty ads.");
            // Fail soft: return an empty AdResponse
            AdResponse reply = AdResponse.newBuilder().build();
            responseObserver.onNext(reply);
            responseObserver.onCompleted();
        } catch (Exception e) {
            // Catch any other unexpected exceptions
            System.err.println("AdService.getAds encountered unexpected error: " + e.getMessage() + ". Returning empty ads.");
            AdResponse reply = AdResponse.newBuilder().build();
            responseObserver.onNext(reply);
            responseObserver.onCompleted();
        }
    }

    // ... existing checkAdFailure method ...
    private void checkAdFailure() {
        // ... existing flagd evaluation logic ...
        if (flagdClient.getBooleanEvaluation("adFailure", false)) {
            throw Status.UNAVAILABLE.withDescription("AdService unavailable due to adFailure flag").asRuntimeException();
        }
    }
}
```

## Recommended follow-up
1.  **Verify flag state:** Confirm that the `adFailure` flag remains disabled in flagd to prevent re-occurrence of this test-injected fault.
2.  **Implement soft-fail logic:** Prioritize implementing the code-level fix described above in the `AdService` to ensure that non-critical features like ads degrade gracefully (return empty ads) instead of propagating gRPC errors, even if a fault flag is accidentally enabled or a real downstream issue occurs. This will prevent future error budget burn for non-essential functionality.
3.  **Review testing practices:** Ensure that fault injection tests using `adFailure` are properly contained and automatically disabled after test runs, or that the alert is configured to be ignored during planned test windows.

## Related prior incidents (cite their IDs)
(no closely related prior incidents found)

---
_Generated by the OmniObserve RCA copilot · model: `gemini-2.5-flash`_
